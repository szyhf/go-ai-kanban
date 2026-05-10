package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// registerScratchRoutes registers scratch-related routes.
func (h *Handler) registerScratchRoutes(r chi.Router) {
	r.Get("/", h.listScratch)
	r.Route("/{type}/{id}", func(r chi.Router) {
		r.Get("/", h.getScratch)
		r.Post("/", h.createScratch)
		r.Put("/", h.upsertScratch)
		r.Delete("/", h.deleteScratch)
		r.Get("/stream/ws", h.handleScratchStreamWS)
	})
}

// listScratch handles GET /api/scratch.
func (h *Handler) listScratch(w http.ResponseWriter, r *http.Request) {
	scratchType := getQuery(r, "type")
	if scratchType == "" {
		scratchType = getQuery(r, "scratch_type")
	}

	if scratchType == "" {
		// Return all scratch entries grouped by type.
		allTypes := []domain.ScratchType{
			domain.ScratchTypeDraftTask,
			domain.ScratchTypeDraftFollowUp,
			domain.ScratchTypeDraftWorkspace,
			domain.ScratchTypeDraftIssue,
			domain.ScratchTypePreviewSettings,
			domain.ScratchTypeWorkspaceNotes,
			domain.ScratchTypeUIPreferences,
			domain.ScratchTypeProjectRepoDefaults,
		}
		result := make(map[string][]domain.Scratch)
		for _, st := range allTypes {
			entries, err := h.scratchRepo.FindAllByType(st)
			if err != nil {
				continue
			}
			if len(entries) > 0 {
				result[string(st)] = entries
			}
		}
		success(w, result)
		return
	}

	st := domain.ScratchType(scratchType)
	entries, err := h.scratchRepo.FindAllByType(st)
	if err != nil {
		internalError(w, "failed to list scratch: "+err.Error())
		return
	}
	success(w, entries)
}

// getScratch handles GET /api/scratch/{type}/{id}.
func (h *Handler) getScratch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	scratchType := domain.ScratchType(chi.URLParam(r, "type"))
	scratch, err := h.scratchRepo.FindByIDAndType(id, scratchType)
	if err != nil || scratch == nil {
		notFound(w, "scratch not found")
		return
	}
	success(w, scratch)
}

// createScratch handles POST /api/scratch/{type}/{id}.
func (h *Handler) createScratch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	scratchType := domain.ScratchType(chi.URLParam(r, "type"))
	var payload domain.ScratchPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	payload.Type = scratchType

	scratch := &domain.Scratch{
		ID:      id,
		Payload: payload,
	}
	if err := h.scratchRepo.Upsert(scratch); err != nil {
		internalError(w, "failed to create scratch: "+err.Error())
		return
	}

	h.eventSvc.NotifyChange(service.HookTableScratch, service.HookOpInsert, id)
	created(w, scratch)
}

// upsertScratch handles PUT /api/scratch/{type}/{id}.
func (h *Handler) upsertScratch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	scratchType := domain.ScratchType(chi.URLParam(r, "type"))
	var payload domain.ScratchPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	payload.Type = scratchType

	scratch := &domain.Scratch{
		ID:      id,
		Payload: payload,
	}
	if err := h.scratchRepo.Upsert(scratch); err != nil {
		internalError(w, "failed to upsert scratch: "+err.Error())
		return
	}

	h.eventSvc.NotifyChange(service.HookTableScratch, service.HookOpUpdate, id)
	success(w, scratch)
}

// deleteScratch handles DELETE /api/scratch/{type}/{id}.
func (h *Handler) deleteScratch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	scratchType := domain.ScratchType(chi.URLParam(r, "type"))
	if err := h.scratchRepo.Delete(id, scratchType); err != nil {
		internalError(w, "failed to delete scratch: "+err.Error())
		return
	}

	h.eventSvc.NotifyDelete(service.HookTableScratch, id)
	success(w, map[string]string{"status": "deleted"})
}
