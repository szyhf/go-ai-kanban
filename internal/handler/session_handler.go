package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// registerSessionRoutes registers session-related routes.
func (h *Handler) registerSessionRoutes(r chi.Router) {
	r.Get("/", h.listSessions)
	r.Post("/", h.createSession)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getSession)
		r.Put("/", h.updateSession)
		r.Get("/queue", h.getQueue)
		r.Post("/queue", h.queueMessage)
		r.Delete("/queue", h.cancelQueued)
	})
}

// listSessions handles GET /api/sessions.
func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	wsIDStr := getQuery(r, "workspace_id")
	if wsIDStr == "" {
		badRequest(w, "workspace_id is required")
		return
	}

	wsID, err := domain.ParseUUID(wsIDStr)
	if err != nil {
		badRequest(w, "invalid workspace_id")
		return
	}

	sessions, err := h.sessionRepo.FindByWorkspaceID(wsID)
	if err != nil {
		internalError(w, "failed to list sessions: "+err.Error())
		return
	}
	success(w, sessions)
}

// createSession handles POST /api/sessions.
func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string  `json:"workspace_id"`
		Executor    *string `json:"executor"`
		Name        *string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.WorkspaceID == "" {
		badRequest(w, "workspace_id is required")
		return
	}

	wsID, err := domain.ParseUUID(req.WorkspaceID)
	if err != nil {
		badRequest(w, "invalid workspace_id")
		return
	}

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: wsID,
		Executor:    req.Executor,
		Name:        req.Name,
	}
	if err := h.sessionRepo.Create(session); err != nil {
		internalError(w, "failed to create session: "+err.Error())
		return
	}
	created(w, session)
}

// getSession handles GET /api/sessions/{id}.
func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	session, err := h.sessionRepo.FindByID(id)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}
	success(w, session)
}

// updateSession handles PUT /api/sessions/{id}.
func (h *Handler) updateSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	session, err := h.sessionRepo.FindByID(id)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	var req struct {
		Executor *string `json:"executor"`
		Name     *string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Executor != nil {
		session.Executor = req.Executor
	}
	if req.Name != nil {
		session.Name = req.Name
	}

	if err := h.sessionRepo.Update(session); err != nil {
		internalError(w, "failed to update session: "+err.Error())
		return
	}
	success(w, session)
}

// getQueue handles GET /api/sessions/{id}/queue.
func (h *Handler) getQueue(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	status := h.queueSvc.GetStatus(id)
	success(w, status)
}

// queueMessage handles POST /api/sessions/{id}/queue.
func (h *Handler) queueMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Prompt == "" {
		badRequest(w, "prompt is required")
		return
	}

	msg := h.queueSvc.QueueMessage(id, domain.DraftFollowUpData{Message: req.Prompt})
	created(w, msg)
}

// cancelQueued handles DELETE /api/sessions/{id}/queue.
func (h *Handler) cancelQueued(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	h.queueSvc.CancelQueued(id)
	success(w, map[string]string{"status": "cancelled"})
}
