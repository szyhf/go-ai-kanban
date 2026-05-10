package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// handleWorkspaceStreamWS handles GET /api/workspaces/streams/ws?archived=false
func (h *Handler) handleWorkspaceStreamWS(w http.ResponseWriter, r *http.Request) {
	archived := r.URL.Query().Get("archived") == "true"

	workspaces, err := h.wsRepo.FindAllWithStatus(nil, nil)
	if err != nil {
		internalError(w, "failed to load workspaces: "+err.Error())
		return
	}

	// Filter by archived status if needed.
	filtered := workspaces
	if !archived {
		var active []domain.WorkspaceWithStatus
		for _, ws := range workspaces {
			if !ws.Archived {
				active = append(active, ws)
			}
		}
		filtered = active
	}

	snapshot := []service.LogMsg{
		service.NewPatchLogMsg(service.WorkspaceSnapshotPatch(filtered)),
	}

	cfg := wsStreamConfig{
		InitialMessages: snapshot,
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return h.eventSvc.FilteredSubscribe("/workspaces")
		},
		Convert: defaultWSConvert,
	}
	handleWSStream(w, r, cfg)
}

// handleExecProcessStreamWS handles GET /api/execution-processes/stream/session/ws?session_id=X
func (h *Handler) handleExecProcessStreamWS(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.URL.Query().Get("session_id")
	if sessionIDStr == "" {
		badRequest(w, "session_id is required")
		return
	}

	sessionID, err := domain.ParseUUID(sessionIDStr)
	if err != nil {
		badRequest(w, "invalid session_id")
		return
	}

	processes, err := h.execRepo.FindBySessionID(sessionID, false)
	if err != nil {
		internalError(w, "failed to load execution processes: "+err.Error())
		return
	}

	snapshot := []service.LogMsg{
		service.NewPatchLogMsg(service.ExecutionProcessSnapshotPatch(processes)),
	}

	cfg := wsStreamConfig{
		InitialMessages: snapshot,
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return h.eventSvc.FilteredSubscribe("/execution_processes")
		},
		Convert: defaultWSConvert,
	}
	handleWSStream(w, r, cfg)
}

// handleScratchStreamWS handles GET /api/scratch/{type}/{id}/stream/ws
func (h *Handler) handleScratchStreamWS(w http.ResponseWriter, r *http.Request) {
	scratchIDStr := chiURLParam(r, "id")
	scratchTypeStr := chiURLParam(r, "type")

	scratchID, err := domain.ParseUUID(scratchIDStr)
	if err != nil {
		badRequest(w, "invalid scratch id")
		return
	}

	scratchType := domain.ScratchType(scratchTypeStr)

	// Load current scratch state.
	scratch, _ := h.scratchRepo.FindByIDAndType(scratchID, scratchType)

	var snapshot []service.LogMsg
	if scratch != nil {
		snapshot = []service.LogMsg{
			service.NewPatchLogMsg(service.ScratchPatch("replace", scratch)),
		}
	}

	cfg := wsStreamConfig{
		InitialMessages: snapshot,
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return h.eventSvc.FilteredSubscribe("/scratch")
		},
		Convert: defaultWSConvert,
	}
	handleWSStream(w, r, cfg)
}

// handleApprovalStreamWS handles GET /api/approvals/stream/ws
func (h *Handler) handleApprovalStreamWS(w http.ResponseWriter, r *http.Request) {
	// Currently using NoopApprovalService which auto-approves everything.
	// Send empty snapshot.
	snapshot := []service.LogMsg{
		service.NewPatchLogMsg(service.ApprovalsSnapshotPatch(map[string]any{})),
	}

	cfg := wsStreamConfig{
		InitialMessages: snapshot,
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return h.eventSvc.FilteredSubscribe("/pending")
		},
		Convert: defaultWSConvert,
	}
	handleWSStream(w, r, cfg)
}

// chiURLParam gets a URL parameter from chi context.
func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
