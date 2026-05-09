package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// registerTagRoutes registers tag-related routes.
func (h *Handler) registerTagRoutes(r chi.Router) {
	r.Get("/", h.listTags)
	r.Post("/", h.createTag)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getTag)
		r.Put("/", h.updateTag)
		r.Delete("/", h.deleteTag)
	})
}

// listTags handles GET /api/tags.
func (h *Handler) listTags(w http.ResponseWriter, _ *http.Request) {
	tags, err := h.tagRepo.FindAll()
	if err != nil {
		internalError(w, "failed to list tags: "+err.Error())
		return
	}
	success(w, tags)
}

// createTag handles POST /api/tags.
func (h *Handler) createTag(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateTag
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TagName == "" {
		badRequest(w, "tag_name is required")
		return
	}

	tag := &domain.Tag{
		ID:      domain.NewUUID(),
		TagName: req.TagName,
		Content: req.Content,
	}
	if err := h.tagRepo.Create(tag); err != nil {
		internalError(w, "failed to create tag: "+err.Error())
		return
	}
	created(w, tag)
}

// getTag handles GET /api/tags/{id}.
func (h *Handler) getTag(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	tag, err := h.tagRepo.FindByID(id)
	if err != nil || tag == nil {
		notFound(w, "tag not found")
		return
	}
	success(w, tag)
}

// updateTag handles PUT /api/tags/{id}.
func (h *Handler) updateTag(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	tag, err := h.tagRepo.FindByID(id)
	if err != nil || tag == nil {
		notFound(w, "tag not found")
		return
	}

	var req domain.UpdateTag
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TagName != nil {
		tag.TagName = *req.TagName
	}
	if req.Content != nil {
		tag.Content = *req.Content
	}

	if err := h.tagRepo.Update(tag); err != nil {
		internalError(w, "failed to update tag: "+err.Error())
		return
	}
	success(w, tag)
}

// deleteTag handles DELETE /api/tags/{id}.
func (h *Handler) deleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.tagRepo.Delete(id); err != nil {
		internalError(w, "failed to delete tag: "+err.Error())
		return
	}
	success(w, map[string]string{"status": "deleted"})
}
