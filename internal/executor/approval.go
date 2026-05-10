package executor

import "context"

// NoopApprovalService auto-approves everything without user interaction.
type NoopApprovalService struct{}

// NewNoopApprovalService creates an approval service that auto-approves all requests.
func NewNoopApprovalService() *NoopApprovalService {
	return &NoopApprovalService{}
}

// CreateToolApproval immediately returns a dummy approval ID.
func (n *NoopApprovalService) CreateToolApproval(toolName string, input string) (string, error) {
	return "auto-approved", nil
}

// WaitToolApproval immediately returns an allow result.
func (n *NoopApprovalService) WaitToolApproval(_ context.Context, _ string) (ApprovalResult, error) {
	return ApprovalResult{Behavior: "allow"}, nil
}
