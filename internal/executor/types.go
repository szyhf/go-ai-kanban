package executor

import (
	"context"
	"io"
	"os/exec"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// ExitResult holds the outcome of a process exit.
type ExitResult struct {
	Code  int
	Error error
}

// SpawnedProcess wraps an OS process with control channels.
// It is the Go equivalent of Rust's SpawnedChild.
type SpawnedProcess struct {
	// Cmd is the underlying OS process handle.
	Cmd *exec.Cmd
	// Stdin is the write end of the process's stdin pipe.
	Stdin io.WriteCloser
	// StdoutPipe is the read end of the process's stdout pipe.
	StdoutPipe io.ReadCloser
	// StderrPipe is the read end of the process's stderr pipe.
	StderrPipe io.ReadCloser
	// Cancel triggers graceful shutdown (sends interrupt to agent).
	Cancel context.CancelFunc
	// Done receives the exit result when the process completes.
	Done <-chan ExitResult
	// RawLines carries raw stdout lines for logging, separate from protocol parsing.
	RawLines <-chan string
}

// ApprovalResult represents the outcome of a tool approval decision.
type ApprovalResult struct {
	Behavior string
	Deny     bool
}

// QuestionResult holds the answer to a user question from the agent.
type QuestionResult struct {
	Answer string
	Deny   bool
}

// ExecutorEnv holds environment variables and context for execution.
type ExecutorEnv struct {
	// EnvVars are additional environment variables to set.
	EnvVars []string
	// RepoPaths are the workspace repo paths.
	RepoPaths []string
	// WorkspaceID is the current workspace UUID.
	WorkspaceID domain.UUID
	// WorkspaceBranch is the workspace branch name.
	WorkspaceBranch string
}

// ToEnvSlice returns the environment as a slice of KEY=VALUE strings.
func (e ExecutorEnv) ToEnvSlice() []string {
	env := make([]string, 0, len(e.EnvVars)+2)
	env = append(env, e.EnvVars...)
	if e.WorkspaceID != (domain.UUID{}) {
		env = append(env, "VK_WORKSPACE_ID="+e.WorkspaceID.String())
	}
	if e.WorkspaceBranch != "" {
		env = append(env, "VK_WORKSPACE_BRANCH="+e.WorkspaceBranch)
	}
	return env
}
