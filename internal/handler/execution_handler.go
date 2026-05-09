package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// registerExecutionRoutes registers execution process-related routes.
func (h *Handler) registerExecutionRoutes(r chi.Router) {
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getExecutionProcess)
		r.Post("/stop", h.stopExecutionProcess)
		r.Get("/repo-states", h.getRepoStates)
	})
}

// getExecutionProcess handles GET /api/execution-processes/{id}.
func (h *Handler) getExecutionProcess(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ep, err := h.execRepo.FindByID(id)
	if err != nil || ep == nil {
		notFound(w, "execution process not found")
		return
	}
	success(w, ep)
}

// stopExecutionProcess handles POST /api/execution-processes/{id}/stop.
func (h *Handler) stopExecutionProcess(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ep, err := h.execRepo.FindByID(id)
	if err != nil || ep == nil {
		notFound(w, "execution process not found")
		return
	}

	if ep.Status != domain.ExecStatusRunning {
		badRequest(w, "execution process is not running")
		return
	}

	// Mark as killed with nil exit code.
	if err := h.execRepo.UpdateStatus(id, domain.ExecStatusKilled, nil); err != nil {
		internalError(w, "failed to stop execution process: "+err.Error())
		return
	}

	success(w, map[string]string{"status": "stopped"})
}

// getRepoStates handles GET /api/execution-processes/{id}/repo-states.
func (h *Handler) getRepoStates(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	states, err := h.execStateRepo.FindByExecutionProcessID(id)
	if err != nil {
		internalError(w, "failed to get repo states: "+err.Error())
		return
	}
	success(w, states)
}
