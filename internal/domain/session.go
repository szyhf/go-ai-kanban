package domain

import "time"

// Session represents an executor session within a workspace.
type Session struct {
	ID              UUID      `json:"id"`
	WorkspaceID     UUID      `json:"workspace_id"`
	Name            *string   `json:"name"`
	Executor        *string   `json:"executor"`
	AgentWorkingDir *string   `json:"agent_working_dir"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateSession is the request DTO for creating a session.
type CreateSession struct {
	Executor *string `json:"executor,omitempty"`
	Name     *string `json:"name,omitempty"`
}
