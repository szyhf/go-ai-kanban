package git

import (
	"fmt"
	"strings"
)

// OpError wraps an error with operation and path context.
type OpError struct {
	Op   string // Operation name (e.g. "open_repo", "create_branch")
	Path string // Repository or worktree path
	Err  error  // Underlying error
}

func (e *OpError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("git %s %s: %v", e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("git %s: %v", e.Op, e.Err)
}

func (e *OpError) Unwrap() error { return e.Err }

// Sentinel errors.
var (
	ErrInvalidRepo     = fmt.Errorf("invalid repository")
	ErrBranchNotFound  = fmt.Errorf("branch not found")
	ErrMergeConflicts  = fmt.Errorf("merge conflicts detected")
	ErrBranchesDiverged = fmt.Errorf("branches have diverged")
	ErrWorktreeDirty   = fmt.Errorf("worktree has uncommitted changes")
	ErrRebaseInProgress = fmt.Errorf("rebase already in progress")
)

// MergeConflictError reports merge conflicts with file details.
type MergeConflictError struct {
	Message string
	Files   []string
}

func (e *MergeConflictError) Error() string {
	if len(e.Files) > 0 {
		return fmt.Sprintf("%s: %s", e.Message, strings.Join(e.Files, ", "))
	}
	return e.Message
}

// DirtyWorktreeError reports a dirty worktree with affected files.
type DirtyWorktreeError struct {
	Branch string
	Files  string
}

func (e *DirtyWorktreeError) Error() string {
	return fmt.Sprintf("worktree dirty on branch %q: %s", e.Branch, e.Files)
}

// CLIError represents a git command failure.
type CLIError struct {
	Command string // The git command that failed
	Stderr  string // Captured stderr output
}

func (e *CLIError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git %s: %s", e.Command, e.Stderr)
	}
	return fmt.Sprintf("git %s failed", e.Command)
}
