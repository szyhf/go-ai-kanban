package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerAgentRoutes registers agent-related routes.
func (h *Handler) registerAgentRoutes(r chi.Router) {
	r.Get("/preset-options", h.handleAgentPresetOptions)
	r.Get("/discovered-options/ws", h.handleAgentDiscoveredOptionsWS)
	r.Get("/check-availability", h.handleAgentCheckAvailability)
}

// handleAgentPresetOptions handles GET /api/agents/preset-options.
func (h *Handler) handleAgentPresetOptions(w http.ResponseWriter, r *http.Request) {
	executor := getQuery(r, "executor")
	_ = executor

	// Return default Claude Code executor config.
	success(w, map[string]interface{}{
		"executor": "CLAUDE_CODE",
		"variant":  nil,
	})
}

// handleAgentDiscoveredOptionsWS handles GET /api/agents/discovered-options/ws.
func (h *Handler) handleAgentDiscoveredOptionsWS(w http.ResponseWriter, r *http.Request) {
	// For now, just handle as a regular HTTP response.
	// WebSocket upgrade is optional for this endpoint.
	success(w, map[string]interface{}{})
}

// handleAgentCheckAvailability handles GET /api/agents/check-availability.
func (h *Handler) handleAgentCheckAvailability(w http.ResponseWriter, r *http.Request) {
	executor := getQuery(r, "executor")
	_ = executor

	success(w, map[string]interface{}{
		"available":   true,
		"version":     nil,
		"install_hint": nil,
	})
}
