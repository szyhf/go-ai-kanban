package domain

import (
	"encoding/json"
	"time"
)

// ExecutionProcess represents a running or completed execution (script, agent, etc.).
type ExecutionProcess struct {
	ID             UUID            `json:"id"`
	SessionID      UUID            `json:"session_id"`
	RunReason      RunReason       `json:"run_reason"`
	ExecutorAction json.RawMessage `json:"executor_action"`
	Status         ExecStatus      `json:"status"`
	ExitCode       *int64          `json:"exit_code"`
	Dropped        bool            `json:"dropped"`
	StartedAt      time.Time       `json:"started_at"`
	CompletedAt    *time.Time      `json:"completed_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateExecutionProcess is the request DTO for creating an execution process.
type CreateExecutionProcess struct {
	SessionID      UUID            `json:"session_id"`
	ExecutorAction json.RawMessage `json:"executor_action"`
	RunReason      RunReason       `json:"run_reason"`
}

// ExecutionProcessRepoState tracks before/after commit state for each repo.
type ExecutionProcessRepoState struct {
	ID                 UUID      `json:"id"`
	ExecutionProcessID UUID      `json:"execution_process_id"`
	RepoID             UUID      `json:"repo_id"`
	BeforeHeadCommit   *string   `json:"before_head_commit"`
	AfterHeadCommit    *string   `json:"after_head_commit"`
	MergeCommit        *string   `json:"merge_commit"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateExecutionProcessRepoState is the internal DTO for creating repo state.
type CreateExecutionProcessRepoState struct {
	RepoID           UUID
	BeforeHeadCommit *string
	AfterHeadCommit  *string
	MergeCommit      *string
}

// CodingAgentTurn represents a single turn in an AI agent conversation.
type CodingAgentTurn struct {
	ID                 UUID      `json:"id"`
	ExecutionProcessID UUID      `json:"execution_process_id"`
	AgentSessionID     *string   `json:"agent_session_id"`
	AgentMessageID     *string   `json:"agent_message_id"`
	Prompt             *string   `json:"prompt"`
	Summary            *string   `json:"summary"`
	Seen               bool      `json:"seen"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateCodingAgentTurn is the request DTO for creating a turn.
type CreateCodingAgentTurn struct {
	ExecutionProcessID UUID    `json:"execution_process_id"`
	Prompt             *string `json:"prompt,omitempty"`
}

// CodingAgentResumeInfo holds info needed to resume an agent session.
type CodingAgentResumeInfo struct {
	SessionID string
	MessageID *string
}

// ExecutionProcessLogs stores raw log output for an execution process.
type ExecutionProcessLogs struct {
	ExecutionID UUID      `json:"execution_id"`
	Logs        string    `json:"logs"`
	ByteSize    int64     `json:"byte_size"`
	InsertedAt  time.Time `json:"inserted_at"`
}

// ExecutionContext groups execution process with its context.
type ExecutionContext struct {
	ExecutionProcess ExecutionProcess
	Session          Session
	Workspace        Workspace
	Repos            []Repo
}

// LatestProcessInfo holds the latest execution process info for a workspace.
type LatestProcessInfo struct {
	WorkspaceID        UUID
	ExecutionProcessID UUID
	SessionID          UUID
	Status             ExecStatus
	CompletedAt        *time.Time
}

// MissingBeforeContext tracks processes that need before-context snapshots.
type MissingBeforeContext struct {
	ID                  UUID
	SessionID           UUID
	WorkspaceID         UUID
	RepoID              UUID
	PrevAfterHeadCommit *string
	TargetBranch        string
	RepoPath            *string
}
