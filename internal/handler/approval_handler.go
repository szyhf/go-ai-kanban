package handler

import (
	"net/http"
)

// handleApprovalRespond handles POST /api/approvals/{id}/respond.
// In local mode with auto-approve, this is a no-op that acknowledges the response.
func (h *Handler) handleApprovalRespond(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		Approved bool   `json:"approved"`
		Behavior string `json:"behavior"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// In local mode, approvals are auto-handled.
	// This endpoint exists for API compatibility with the frontend.
	_ = id

	success(w, map[string]string{
		"status":   "responded",
		"approved": boolToStr(req.Approved),
	})
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
