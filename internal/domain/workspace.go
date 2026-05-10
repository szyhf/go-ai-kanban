package domain

import "time"

// Workspace represents a working branch/context for a task.
type Workspace struct {
	ID               UUID       `json:"id"`
	TaskID           *UUID      `json:"task_id"`
	ContainerRef     *string    `json:"container_ref"`
	Branch           string     `json:"branch"`
	SetupCompletedAt *time.Time `json:"setup_completed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Archived         bool       `json:"archived"`
	Pinned           bool       `json:"pinned"`
	Name             *string    `json:"name"`
	WorktreeDeleted  bool       `json:"worktree_deleted"`
}

// WorkspaceWithStatus embeds Workspace and adds runtime status.
type WorkspaceWithStatus struct {
	Workspace
	IsRunning bool `json:"is_running"`
	IsErrored bool `json:"is_errored"`
}

// CreateWorkspace is the request DTO for creating a workspace.
type CreateWorkspace struct {
	Branch string  `json:"branch"`
	Name   *string `json:"name,omitempty"`
}

// CreateFollowUpAttempt is the request DTO for creating a follow-up.
type CreateFollowUpAttempt struct {
	Prompt string `json:"prompt"`
}

// WorkspaceContext groups a workspace with its repos and active session.
type WorkspaceContext struct {
	Workspace             Workspace
	WorkspaceRepos        []RepoWithTargetBranch
	OrchestratorSessionID *UUID
}

// ContainerInfo represents container metadata for a workspace.
type ContainerInfo struct {
	WorkspaceID UUID `json:"workspace_id"`
}
