package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerMCPRoutes registers MCP configuration routes.
func (h *Handler) registerMCPRoutes(r chi.Router) {
	r.Get("/", h.handleGetMCPConfig)
	r.Post("/", h.handleSaveMCPConfig)
}

// handleGetMCPConfig handles GET /api/mcp-config.
func (h *Handler) handleGetMCPConfig(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"servers": []any{},
	})
}

// handleSaveMCPConfig handles POST /api/mcp-config.
func (h *Handler) handleSaveMCPConfig(w http.ResponseWriter, r *http.Request) {
	// Drain the body but don't persist.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
