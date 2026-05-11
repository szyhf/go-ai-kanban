package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/http"

	"github.com/xuzhiping7/ai-kanban/relay/internal/auth"
)

// handleSigningSessionRefresh handles POST /api/relay-auth/server/signing-session/refresh.
// Request:  {"client_id":"...", "timestamp":123, "nonce":"...", "signature_b64":"..."}
// Response: {"success": true, "data": {"signing_session_id": "..."}}
func (s *Server) handleSigningSessionRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID     string `json:"client_id"`
		Timestamp    int64  `json:"timestamp"`
		Nonce        string `json:"nonce"`
		SignatureB64 string `json:"signature_b64"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.ClientID == "" || req.Nonce == "" || req.SignatureB64 == "" {
		fail(w, "client_id, nonce, and signature_b64 are required", http.StatusBadRequest)
		return
	}

	// Get client to retrieve public key.
	client, err := s.store.GetClient(req.ClientID)
	if err != nil {
		fail(w, "failed to get client", http.StatusInternalServerError)
		return
	}
	if client == nil {
		fail(w, "client not found", http.StatusNotFound)
		return
	}

	// Check and consume nonce.
	if err := s.sessions.CheckAndConsumeNonce(req.Nonce); err != nil {
		fail(w, "invalid nonce: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Decode signature.
	sig, err := base64.StdEncoding.DecodeString(req.SignatureB64)
	if err != nil {
		fail(w, "invalid signature_b64", http.StatusBadRequest)
		return
	}

	// Build the refresh payload and verify.
	payload := auth.RefreshSigningPayload(req.Timestamp, req.Nonce, req.ClientID)

	if !ed25519.Verify(ed25519.PublicKey(client.PublicKey), []byte(payload), sig) {
		fail(w, "signature verification failed", http.StatusUnauthorized)
		return
	}

	// Refresh the session — look up active session for this client.
	// For now we return an empty session ID since session lookup by client
	// requires additional store support.
	success(w, map[string]string{
		"signing_session_id": "",
	})
}
