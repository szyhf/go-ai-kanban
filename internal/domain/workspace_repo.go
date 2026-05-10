package domain

import "time"

// WorkspaceRepo is a junction between workspace and repo with a target branch.
type WorkspaceRepo struct {
	ID           UUID      `json:"id"`
	WorkspaceID  UUID      `json:"workspace_id"`
	RepoID       UUID      `json:"repo_id"`
	TargetBranch string    `json:"target_branch"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateWorkspaceRepo is the request DTO for adding a repo to a workspace.
type CreateWorkspaceRepo struct {
	RepoID       UUID   `json:"repo_id"`
	TargetBranch string `json:"target_branch"`
}
