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

// executorActionWrapper handles the ExecutorAction JSON format:
//   {"typ": {...}, "next_action": null}
// The field name is `typ` (not `type`) and wraps the enum variant under it.
type executorActionWrapper struct {
	Typ        json.RawMessage `json:"typ"`
	NextAction json.RawMessage `json:"next_action,omitempty"`
}

// executorActionTypeTag extracts the "type" discriminant from an action type object.
type executorActionTypeTag struct {
	Type string `json:"type"`
}

// ParseExecutorAction unmarshals a raw JSON ExecutorAction and returns the action type.
func ParseExecutorAction(raw json.RawMessage) (*ExecutorAction, error) {
	var wrapper executorActionWrapper
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}

	var tag executorActionTypeTag
	if err := json.Unmarshal(wrapper.Typ, &tag); err != nil {
		return nil, err
	}

	return &ExecutorAction{
		Type: ExecutorActionType(tag.Type),
		RawTyp: wrapper.Typ,
	}, nil
}

// ExecutorAction represents a parsed executor action with its type discriminant.
type ExecutorAction struct {
	Type    ExecutorActionType
	RawTyp  json.RawMessage
}

// CodingAgentInitialRequest is the payload for a fresh coding agent invocation.
type CodingAgentInitialRequest struct {
	Prompt         string          `json:"prompt"`
	ExecutorConfig ExecutorConfig  `json:"executor_config"`
	WorkingDir     *string         `json:"working_dir,omitempty"`
}

// CodingAgentFollowUpRequest is the payload for resuming a coding agent session.
type CodingAgentFollowUpRequest struct {
	Prompt           string          `json:"prompt"`
	SessionID        string          `json:"session_id"`
	ResetToMessageID *string         `json:"reset_to_message_id,omitempty"`
	ExecutorConfig   ExecutorConfig  `json:"executor_config"`
	WorkingDir       *string         `json:"working_dir,omitempty"`
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

// ExtractCodingAgentFromAction returns the BaseCodingAgent for an action's executor_config.
func ExtractCodingAgentFromAction(raw json.RawMessage) BaseCodingAgent {
	inner := UnwrapActionTyp(raw)
	if inner == nil {
		return AgentClaudeCode
	}

	// Try initial request first.
	var initial CodingAgentInitialRequest
	if err := json.Unmarshal(inner, &initial); err == nil && initial.ExecutorConfig.Executor != "" {
		return initial.ExecutorConfig.Executor
	}
	// Try follow-up request.
	var followUp CodingAgentFollowUpRequest
	if err := json.Unmarshal(inner, &followUp); err == nil && followUp.ExecutorConfig.Executor != "" {
		return followUp.ExecutorConfig.Executor
	}
	return AgentClaudeCode
}

// UnwrapActionTyp extracts the inner "typ" JSON from an ExecutorAction wrapper.
// If the JSON doesn't have a "typ" field, returns the raw input as-is (backward compatible).
func UnwrapActionTyp(raw json.RawMessage) json.RawMessage {
	var wrapper executorActionWrapper
	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Typ == nil {
		// Not wrapped — return as-is for backward compatibility.
		return raw
	}
	return wrapper.Typ
}

// BuildExecutorActionJSON builds the ExecutorAction JSON in the correct format:
//   {"typ": {...action_type_object...}, "next_action": null}
func BuildExecutorActionJSON(actionTypeObj interface{}) (json.RawMessage, error) {
	typBytes, err := json.Marshal(actionTypeObj)
	if err != nil {
		return nil, err
	}
	wrapper := executorActionWrapper{
		Typ: json.RawMessage(typBytes),
	}
	result, err := json.Marshal(wrapper)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}
