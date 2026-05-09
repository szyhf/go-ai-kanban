package git

import "time"

// Default identity used for commits when no user identity is configured.
const (
	DefaultUserName  = "Vibe Kanban"
	DefaultUserEmail = "noreply@vibekanban.com"
)

// Commit represents a git commit identified by its SHA hash.
type Commit struct {
	Hash string
}

// HeadInfo contains information about the current HEAD.
type HeadInfo struct {
	Branch string // Branch name (empty if detached)
	OID    string // Commit SHA
}

// GitBranch represents a git branch with metadata.
type GitBranch struct {
	Name           string    `json:"name"`
	IsCurrent      bool      `json:"isCurrent"`
	IsRemote       bool      `json:"isRemote"`
	LastCommitDate time.Time `json:"lastCommitDate"`
}

// GitRemote represents a git remote.
type GitRemote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ConflictOp identifies what operation caused a conflict.
type ConflictOp string

const (
	ConflictOpRebase     ConflictOp = "rebase"
	ConflictOpMerge      ConflictOp = "merge"
	ConflictOpCherryPick ConflictOp = "cherry_pick"
	ConflictOpRevert     ConflictOp = "revert"
)

// WorktreeResetOptions controls worktree reset behavior.
type WorktreeResetOptions struct {
	PerformReset     bool
	ForceWhenDirty   bool
	IsDirty          bool
	LogSkipWhenDirty bool
}

// WorktreeResetOutcome reports the result of a worktree reset attempt.
type WorktreeResetOutcome struct {
	Needed  bool
	Applied bool
}
