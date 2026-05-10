package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

const defaultClaudeCodeVersion = "@anthropic-ai/claude-code@2.1.119"

// ClaudeCodeExecutor implements Executor for the Claude Code CLI.
type ClaudeCodeExecutor struct {
	Plan                       bool
	Approvals                  bool
	Model                      *string
	Effort                     *string
	Agent                      *string
	DangerouslySkipPermissions bool
	Router                     bool
	PermissionPolicy           *domain.PermissionPolicy
	logger                     *slog.Logger
}

// NewClaudeCodeExecutor creates a Claude Code executor from an ExecutorConfig.
func NewClaudeCodeExecutor(config *domain.ExecutorConfig) *ClaudeCodeExecutor {
	e := &ClaudeCodeExecutor{
		logger: slog.Default(),
	}
	if config == nil {
		return e
	}
	if config.ModelID != nil {
		e.Model = config.ModelID
	}
	if config.AgentID != nil {
		e.Agent = config.AgentID
	}
	if config.ReasoningID != nil {
		e.Effort = config.ReasoningID
	}
	if config.PermissionPolicy != nil {
		e.PermissionPolicy = config.PermissionPolicy
	}
	return e
}

// baseCommand returns the npx command prefix for Claude Code.
func (e *ClaudeCodeExecutor) baseCommand() string {
	if e.Router {
		return "npx -y @musistudio/claude-code-router@1.0.66 code"
	}
	return "npx -y " + defaultClaudeCodeVersion
}

// buildArgs constructs the CLI arguments for the Claude Code command.
func (e *ClaudeCodeExecutor) buildArgs(prompt string, resumeSession *string, resumeMessageID *string) []string {
	args := []string{
		"-p", prompt,
		"--verbose",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
	}

	// Permission mode selection.
	if e.Plan || e.Approvals {
		args = append(args,
			"--permission-prompt-tool", "stdio",
			"--permission-mode", string(PermBypassPermissions),
		)
	} else {
		args = append(args, "--disallowedTools", "AskUserQuestion")
	}

	if e.DangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}
	if e.Model != nil && *e.Model != "" {
		args = append(args, "--model", *e.Model)
	}
	if e.Effort != nil && *e.Effort != "" {
		args = append(args, "--effort", *e.Effort)
	}
	if e.Agent != nil && *e.Agent != "" {
		args = append(args, "--agent", *e.Agent)
	}
	if resumeSession != nil && *resumeSession != "" {
		args = append(args, "--resume", *resumeSession)
	}
	if resumeMessageID != nil && *resumeMessageID != "" {
		args = append(args, "--resume-session-at", *resumeMessageID)
	}

	return args
}

// Spawn starts a new Claude Code process.
func (e *ClaudeCodeExecutor) Spawn(ctx context.Context, dir string, prompt string, env *ExecutorEnv) (*SpawnedProcess, error) {
	return e.spawnInternal(ctx, dir, prompt, nil, nil, env)
}

// SpawnFollowUp resumes an existing Claude Code session.
func (e *ClaudeCodeExecutor) SpawnFollowUp(ctx context.Context, dir string, prompt string, sessionID string, resetToMsgID *string, env *ExecutorEnv) (*SpawnedProcess, error) {
	return e.spawnInternal(ctx, dir, prompt, &sessionID, resetToMsgID, env)
}

func (e *ClaudeCodeExecutor) spawnInternal(ctx context.Context, dir string, prompt string, resumeSession *string, resumeMessageID *string, env *ExecutorEnv) (*SpawnedProcess, error) {
	// Build the full command line: "npx -y @anthropic-ai/claude-code@ver" + args
	fullArgs := e.buildArgs(prompt, resumeSession, resumeMessageID)
	cmdLine := e.baseCommand()

	// Parse base command into name + args.
	// The baseCommand returns a shell command string like "npx -y @anthropic-ai/claude-code@2.1.119"
	// We need to split it and append our args.
	baseParts := parseCommandLine(cmdLine)
	allArgs := append(baseParts[1:], fullArgs...)

	var envSlice []string
	if env != nil {
		envSlice = env.ToEnvSlice()
	}

	cmd, err := BuildCommand(dir, baseParts[0], allArgs, envSlice)
	if err != nil {
		return nil, fmt.Errorf("build command: %w", err)
	}

	// Create pipes for stdin/stdout/stderr.
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	e.logger.Info("claude code process started", "pid", cmd.Process.Pid, "dir", dir)

	// Set up cancellation context.
	cancelCtx, cancel := context.WithCancel(ctx)

	// Channel for raw stdout lines (for logging).
	rawLines := make(chan string, 256)
	// Channel for exit result.
	done := make(chan ExitResult, 1)

	// Start exit monitor goroutine.
	go WaitProcess(cancelCtx, cmd, done)

	// Start the stdout line reader that feeds rawLines.
	go readStdoutLines(cancelCtx, stdoutPipe, rawLines)

	// Close resources when context is done.
	go func() {
		<-cancelCtx.Done()
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		close(rawLines)
	}()

	return &SpawnedProcess{
		Cmd:       cmd,
		Stdin:     io.WriteCloser(stdinPipe),
		StdoutPipe: stdoutPipe,
		StderrPipe: stderrPipe,
		Cancel:    cancel,
		Done:      done,
		RawLines:  rawLines,
	}, nil
}

// parseCommandLine splits a command string into parts.
// Simple space-based splitting; does not handle quoting.
func parseCommandLine(cmd string) []string {
	var parts []string
	current := ""
	for _, ch := range cmd {
		if ch == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// readStdoutLines reads lines from stdout and sends them to the rawLines channel.
func readStdoutLines(ctx context.Context, reader io.Reader, lines chan<- string) {
	buf := make([]byte, 4096)
	var pending []byte
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := reader.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			// Split on newlines and send complete lines.
			for {
				idx := findNewline(pending)
				if idx < 0 {
					break
				}
				line := string(pending[:idx])
				pending = pending[idx+1:]
				select {
				case lines <- line:
				case <-ctx.Done():
					return
				}
			}
		}
		if err != nil {
			// Send any remaining partial data.
			if len(pending) > 0 {
				select {
				case lines <- string(pending):
				case <-ctx.Done():
				}
			}
			return
		}
	}
}

func findNewline(data []byte) int {
	for i, b := range data {
		if b == '\n' {
			return i
		}
	}
	return -1
}
