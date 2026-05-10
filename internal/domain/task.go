package domain

import "time"

// Task represents a kanban task within a project.
type Task struct {
	ID                UUID       `json:"id"`
	ProjectID         UUID       `json:"project_id"`
	Title             string     `json:"title"`
	Description       *string    `json:"description"`
	Status            TaskStatus `json:"status"`
	ParentWorkspaceID *UUID      `json:"parent_workspace_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
