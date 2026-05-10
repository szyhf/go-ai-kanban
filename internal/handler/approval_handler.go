package handler

import (
	"net/http"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/executor"
)

// handleApprovalRespond handles POST /api/approvals/{id}/respond.
// When an executor process is running, the decision is routed to it via the
// process store's ApprovalCh. When no process is found (e.g. local mode),
// the endpoint still returns success for API compatibility.
func (h *Handler) handleApprovalRespond(w http.ResponseWriter, r *http.Request) {
	_, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		ExecutionProcessID string `json:"execution_process_id"`
		Status             struct {
			Status  string `json:"status"`
			Reason  string `json:"reason,omitempty"`
			Answers []any  `json:"answers,omitempty"`
		} `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Determine approval decision from the status field.
	approved := req.Status.Status == "approved"
	behavior := "deny"
	if approved {
		behavior = "allow"
	}

	// Attempt to deliver the decision to a running executor process.
	delivered := false
	if h.containerSvc != nil && req.ExecutionProcessID != "" {
		processID, err := domain.ParseUUID(req.ExecutionProcessID)
		if err == nil {
			decision := executor.ApprovalDecision{
				Approved: approved,
				Behavior: behavior,
				Reason:   req.Status.Reason,
			}
			delivered = h.containerSvc.SendApprovalDecision(processID, decision)
		}
	}

	status := "responded"
	if !delivered {
		status = "auto_approved"
	}

	success(w, map[string]string{
		"status":   status,
		"approved": boolToStr(approved),
	})
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
