package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerRelayRoutes registers relay-related routes.
// In local mode, all relay endpoints are stubs (relay requires remote server).
func (h *Handler) registerRelayRoutes(r chi.Router) {
	r.Post("/server/enrollment-code", h.handleRelayEnrollmentCode)
	r.Get("/server/clients", h.handleRelayListClients)
	r.Delete("/server/clients/{clientId}", h.handleRelayRemoveClient)
	r.Post("/client/pair", h.handleRelayPairHost)
	r.Get("/client/hosts", h.handleRelayListHosts)
	r.Delete("/client/hosts/{hostId}", h.handleRelayRemoveHost)
}

// handleRelayEnrollmentCode handles POST /api/relay-auth/server/enrollment-code.
func (h *Handler) handleRelayEnrollmentCode(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"enrollment_code": "",
	})
}

// handleRelayListClients handles GET /api/relay-auth/server/clients.
func (h *Handler) handleRelayListClients(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"clients": []any{},
	})
}

// handleRelayRemoveClient handles DELETE /api/relay-auth/server/clients/{clientId}.
func (h *Handler) handleRelayRemoveClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// handleRelayPairHost handles POST /api/relay-auth/client/pair.
func (h *Handler) handleRelayPairHost(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{})
}

// handleRelayListHosts handles GET /api/relay-auth/client/hosts.
func (h *Handler) handleRelayListHosts(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"hosts": []any{},
	})
}

// handleRelayRemoveHost handles DELETE /api/relay-auth/client/hosts/{hostId}.
func (h *Handler) handleRelayRemoveHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
