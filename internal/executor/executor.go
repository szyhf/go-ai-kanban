package executor

import (
	"context"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// Executor is the core interface for spawning coding agent processes.
// Implemented by ClaudeCodeExecutor and future executors.
type Executor interface {
	// Spawn starts a new agent process with the given prompt.
	Spawn(ctx context.Context, dir string, prompt string, env *ExecutorEnv) (*SpawnedProcess, error)
	// SpawnFollowUp resumes an existing agent session with a new prompt.
	SpawnFollowUp(ctx context.Context, dir string, prompt string, sessionID string, resetToMsgID *string, env *ExecutorEnv) (*SpawnedProcess, error)
}

// ApprovalService abstracts tool/question approval backends.
type ApprovalService interface {
	// CreateToolApproval creates a pending approval for a tool use.
	CreateToolApproval(toolName string, input string) (approvalID string, err error)
	// WaitToolApproval blocks until the user approves or denies a tool approval.
	WaitToolApproval(ctx context.Context, approvalID string) (ApprovalResult, error)
}

// resolveExecutor returns the appropriate Executor for the given agent type.
// For now, only Claude Code is supported.
func resolveExecutor(agent domain.BaseCodingAgent, config *domain.ExecutorConfig) Executor {
	// Default to Claude Code for all agent types in the initial implementation.
	return NewClaudeCodeExecutor(config)
}
