package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerProfileRoutes registers profile configuration routes.
func (h *Handler) registerProfileRoutes(r chi.Router) {
	r.Get("/", h.handleGetProfile)
	r.Put("/", h.handleSaveProfile)
}

// handleGetProfile handles GET /api/profiles.
func (h *Handler) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{
		"content": "",
		"path":    "",
	})
}

// handleSaveProfile handles PUT /api/profiles.
func (h *Handler) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{
		"content": "",
		"path":    "",
	})
}
