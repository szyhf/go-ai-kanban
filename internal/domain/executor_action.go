package domain

import "encoding/json"

// ExecutorActionType identifies the kind of execution action.
type ExecutorActionType string

const (
	ActionCodingAgentInitial  ExecutorActionType = "CodingAgentInitialRequest"
	ActionCodingAgentFollowUp ExecutorActionType = "CodingAgentFollowUpRequest"
	ActionScript              ExecutorActionType = "ScriptRequest"
	ActionReview              ExecutorActionType = "ReviewRequest"
)

// ExecutorAction represents a single execution step, optionally chained to a next action.
// Matches Rust crates/executors/src/actions/mod.rs ExecutorAction.
type ExecutorAction struct {
	Type       ExecutorActionType `json:"type"`
	NextAction *ExecutorAction    `json:"next_action,omitempty"`
}

// CodingAgentInitialRequest is the payload for a fresh coding agent invocation.
// Matches Rust crates/executors/src/actions/coding_agent_initial.rs.
type CodingAgentInitialRequest struct {
	Prompt         string          `json:"prompt"`
	ExecutorConfig ExecutorConfig  `json:"executor_config"`
	WorkingDir     *string         `json:"working_dir,omitempty"`
}

// CodingAgentFollowUpRequest is the payload for resuming a coding agent session.
// Matches Rust crates/executors/src/actions/coding_agent_follow_up.rs.
type CodingAgentFollowUpRequest struct {
	Prompt          string          `json:"prompt"`
	SessionID       string          `json:"session_id"`
	ResetToMessageID *string        `json:"reset_to_message_id,omitempty"`
	ExecutorConfig  ExecutorConfig  `json:"executor_config"`
	WorkingDir      *string         `json:"working_dir,omitempty"`
}

// ScriptRequest is the payload for running a script action.
type ScriptRequest struct {
	Script     string         `json:"script"`
	WorkingDir *string        `json:"working_dir,omitempty"`
}

// ReviewRequest is the payload for a code review action.
type ReviewRequest struct {
	Prompt         string         `json:"prompt"`
	ExecutorConfig ExecutorConfig `json:"executor_config"`
	WorkingDir     *string        `json:"working_dir,omitempty"`
}

// BaseCodingAgent identifies the coding agent type.
type BaseCodingAgent string

const (
	AgentClaudeCode  BaseCodingAgent = "CLAUDE_CODE"
	AgentAmp         BaseCodingAgent = "AMP"
	AgentGemini      BaseCodingAgent = "GEMINI"
	AgentCodex       BaseCodingAgent = "CODEX"
	AgentOpencode    BaseCodingAgent = "OPENCODE"
	AgentCursorAgent BaseCodingAgent = "CURSOR_AGENT"
	AgentQwenCode    BaseCodingAgent = "QWEN_CODE"
	AgentCopilot     BaseCodingAgent = "COPILOT"
	AgentDroid       BaseCodingAgent = "DROID"
)

// PermissionPolicy defines the permission mode for an executor.
type PermissionPolicy string

const (
	PermissionDefault           PermissionPolicy = "default"
	PermissionAcceptEdits       PermissionPolicy = "acceptEdits"
	PermissionPlan              PermissionPolicy = "plan"
	PermissionBypassPermissions PermissionPolicy = "bypassPermissions"
	PermissionAuto              PermissionPolicy = "auto"
)

// ExecutorConfig is the unified executor identity with optional overrides.
// Matches Rust crates/executors/src/profile.rs ExecutorConfig.
type ExecutorConfig struct {
	Executor         BaseCodingAgent   `json:"executor"`
	Variant          *string           `json:"variant,omitempty"`
	ModelID          *string           `json:"model_id,omitempty"`
	AgentID          *string           `json:"agent_id,omitempty"`
	ReasoningID      *string           `json:"reasoning_id,omitempty"`
	PermissionPolicy *PermissionPolicy `json:"permission_policy,omitempty"`
}

// HasOverrides returns true if any override field is set.
func (c ExecutorConfig) HasOverrides() bool {
	return c.ModelID != nil || c.AgentID != nil || c.ReasoningID != nil || c.PermissionPolicy != nil
}

// ParseExecutorAction unmarshals a raw JSON ExecutorAction.
func ParseExecutorAction(raw json.RawMessage) (*ExecutorAction, error) {
	var action ExecutorAction
	if err := json.Unmarshal(raw, &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// ExtractCodingAgentFromAction returns the BaseCodingAgent for an action's executor_config.
func ExtractCodingAgentFromAction(raw json.RawMessage) BaseCodingAgent {
	// Try initial request first.
	var initial CodingAgentInitialRequest
	if err := json.Unmarshal(raw, &initial); err == nil && initial.ExecutorConfig.Executor != "" {
		return initial.ExecutorConfig.Executor
	}
	// Try follow-up request.
	var followUp CodingAgentFollowUpRequest
	if err := json.Unmarshal(raw, &followUp); err == nil && followUp.ExecutorConfig.Executor != "" {
		return followUp.ExecutorConfig.Executor
	}
	return AgentClaudeCode
}
