package executor

import "encoding/json"

// PermissionMode defines the permission mode for Claude Code CLI.
type PermissionMode string

const (
	PermDefault           PermissionMode = "default"
	PermPlan              PermissionMode = "plan"
	PermBypassPermissions PermissionMode = "bypassPermissions"
)

// CLIMessage represents a message from Claude Code CLI stdout.
// Each line is a JSON object with a "type" field.
type CLIMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Request   json.RawMessage `json:"request,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	// Session info from system messages
	SessionID *string `json:"session_id,omitempty"`
	MessageID *string `json:"message_id,omitempty"`
}

// ControlRequest represents a request from Claude Code that needs a response.
type ControlRequest struct {
	Subtype  string          `json:"subtype"`
	ToolName string          `json:"tool_name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	// For can_use_tool
	ToolUseID *string `json:"tool_use_id,omitempty"`
}

// ControlResponse is sent back to Claude Code in response to a control_request.
type ControlResponse struct {
	Type     string             `json:"type"`
	Accepted bool               `json:"accepted,omitempty"`
	Response PermissionResponse `json:"response,omitempty"`
}

// PermissionResponse is the response payload for permission requests.
type PermissionResponse struct {
	Behavior           string             `json:"behavior"`
	UpdatedInput       json.RawMessage    `json:"updatedInput,omitempty"`
	UpdatedPermissions []PermissionUpdate `json:"updatedPermissions,omitempty"`
}

// PermissionUpdate represents a permission grant/deny update.
type PermissionUpdate struct {
	ToolName string `json:"tool_name"`
	Behavior string `json:"behavior"`
}

// UserMessage is sent to Claude Code via stdin to deliver a user prompt.
type UserMessage struct {
	Type    string      `json:"type"`
	Message MessageBody `json:"message"`
}

// MessageBody wraps the role and content of a message.
type MessageBody struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SDKControlRequest is sent to Claude Code via stdin for control operations.
type SDKControlRequest struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
}

// InitializeRequest is the payload for the initialize control request.
type InitializeRequest struct {
	Subtype string          `json:"subtype"`
	Hooks   json.RawMessage `json:"hooks,omitempty"`
}

// SetPermissionModeRequest sets the permission mode.
type SetPermissionModeRequest struct {
	Subtype string         `json:"subtype"`
	Mode    PermissionMode `json:"mode"`
}

// InterruptRequest sends an interrupt signal.
type InterruptRequest struct {
	Subtype string `json:"subtype"`
}

// HookCallbackRequest represents a hook callback from Claude Code.
type HookCallbackRequest struct {
	Subtype    string          `json:"subtype"`
	CallbackID string          `json:"callback_id"`
	Input      json.RawMessage `json:"input,omitempty"`
}

// Well-known callback IDs.
const (
	CallbackStopGitCheck = "STOP_GIT_CHECK_CALLBACK_ID"
	CallbackAutoApprove  = "AUTO_APPROVE_CALLBACK_ID"
)
