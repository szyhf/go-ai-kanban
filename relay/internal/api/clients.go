package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleListClients handles GET /api/relay-auth/server/clients.
func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.store.ListClients()
	if err != nil {
		fail(w, "failed to list clients", http.StatusInternalServerError)
		return
	}

	type relayPairedClient struct {
		ClientID      string `json:"client_id"`
		ClientName    string `json:"client_name"`
		ClientBrowser string `json:"client_browser"`
		ClientOS      string `json:"client_os"`
		ClientDevice  string `json:"client_device"`
	}

	result := make([]relayPairedClient, 0, len(clients))
	for _, c := range clients {
		result = append(result, relayPairedClient{
			ClientID:      c.ID,
			ClientName:    c.DisplayName,
			ClientBrowser: c.DeviceType,
		})
	}

	success(w, map[string]any{
		"clients": result,
	})
}

// handleRemoveClient handles DELETE /api/relay-auth/server/clients/{clientId}.
func (s *Server) handleRemoveClient(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientId")
	if clientID == "" {
		fail(w, "client ID is required", http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteClient(clientID); err != nil {
		fail(w, "failed to remove client", http.StatusInternalServerError)
		return
	}

	noContent(w)
}

// handleListHosts handles GET /api/relay-auth/client/hosts.
func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := clientIDFromContext(r.Context())
	if !ok {
		fail(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	hosts, err := s.store.ListHostsByClient(clientID)
	if err != nil {
		fail(w, "failed to list hosts", http.StatusInternalServerError)
		return
	}

	type relayHost struct {
		ID          string `json:"host_id"`
		DisplayName string `json:"host_name"`
		ClientID    string `json:"client_id"`
	}

	result := make([]relayHost, 0, len(hosts))
	for _, h := range hosts {
		result = append(result, relayHost{
			ID:          h.ID,
			DisplayName: h.DisplayName,
			ClientID:    h.ClientID,
		})
	}

	success(w, map[string]any{
		"hosts": result,
	})
}

// handleRemoveHost handles DELETE /api/relay-auth/client/hosts/{hostId}.
func (s *Server) handleRemoveHost(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "hostId")
	if hostID == "" {
		fail(w, "host ID is required", http.StatusBadRequest)
		return
	}

	if err := s.store.DeleteHost(hostID); err != nil {
		fail(w, "failed to remove host", http.StatusInternalServerError)
		return
	}

	noContent(w)
}
