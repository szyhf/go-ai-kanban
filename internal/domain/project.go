package domain

import "time"

// Project represents a kanban project.
type Project struct {
	ID                   UUID      `json:"id"`
	Name                 string    `json:"name"`
	DefaultAgentWorkingDir *string `json:"default_agent_working_dir"`
	RemoteProjectID      *UUID     `json:"remote_project_id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
