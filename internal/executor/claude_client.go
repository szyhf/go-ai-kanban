package executor

import (
	"context"
	"encoding/json"
	"log/slog"
)

// ClaudeAgentClient handles approval and hook decisions for Claude Code.
type ClaudeAgentClient struct {
	approvals   ApprovalService
	autoApprove bool
	logger      *slog.Logger
}

// NewClaudeAgentClient creates a new client with the given approval service.
// If approvals is nil, all tool uses are auto-approved.
func NewClaudeAgentClient(approvals ApprovalService, autoApprove bool) *ClaudeAgentClient {
	return &ClaudeAgentClient{
		approvals:   approvals,
		autoApprove: autoApprove,
		logger:      slog.Default(),
	}
}

// OnCanUseTool handles a "can_use_tool" control request from Claude Code.
func (c *ClaudeAgentClient) OnCanUseTool(toolName string, input json.RawMessage, toolUseID *string) (ApprovalResult, error) {
	// Auto-approve if configured.
	if c.autoApprove || c.approvals == nil {
		c.logger.Debug("自动批准工具", "tool", toolName)
		return ApprovalResult{Behavior: "allow"}, nil
	}

	// Create a pending approval.
	approvalID, err := c.approvals.CreateToolApproval(toolName, string(input))
	if err != nil {
		return ApprovalResult{Behavior: "deny"}, err
	}

	// Wait for the user to approve or deny.
	result, err := c.approvals.WaitToolApproval(context.Background(), approvalID)
	if err != nil {
		return ApprovalResult{Behavior: "deny"}, err
	}

	return result, nil
}

// OnHookCallback handles a hook callback from Claude Code.
func (c *ClaudeAgentClient) OnHookCallback(callbackID string, input json.RawMessage) (ApprovalResult, error) {
	switch callbackID {
	case CallbackStopGitCheck:
		// Check for uncommitted changes — always allow for now.
		c.logger.Debug("停止 git 检查回调，允许操作")
		return ApprovalResult{Behavior: "allow"}, nil
	case CallbackAutoApprove:
		// Auto-approve callback — always allow.
		return ApprovalResult{Behavior: "allow"}, nil
	default:
		c.logger.Warn("未知的 hook 回调", "callback_id", callbackID)
		return ApprovalResult{Behavior: "allow"}, nil
	}
}
