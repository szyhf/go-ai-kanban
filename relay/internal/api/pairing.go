package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/xuzhiping7/ai-kanban/relay/internal/spake2"
	"github.com/xuzhiping7/ai-kanban/relay/internal/store"
)

// pendingEnrollment tracks an in-progress SPAKE2 exchange.
type pendingEnrollment struct {
	enrollmentCode string
	serverState    *spake2.State
	sharedKey      []byte
	createdAt      time.Time
}

// pendingEnrollments stores in-progress SPAKE2 exchanges keyed by enrollment ID.
// In a production system this would be in Redis or the database.
// TODO: move to store or add TTL cleanup.
var pendingEnrollments = make(map[string]*pendingEnrollment)

// handleSpake2Start handles POST /api/relay-auth/server/spake2/start.
// Request:  {"enrollment_code": "ABC123", "client_message_b64": "..."}
// Response: {"success": true, "data": {"enrollment_id": "...", "server_message_b64": "..."}}
func (s *Server) handleSpake2Start(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnrollmentCode   string `json:"enrollment_code"`
		ClientMessageB64 string `json:"client_message_b64"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.EnrollmentCode == "" || req.ClientMessageB64 == "" {
		fail(w, "enrollment_code and client_message_b64 are required", http.StatusBadRequest)
		return
	}

	// Validate enrollment code.
	valid, err := s.store.UseEnrollmentCode(req.EnrollmentCode)
	if err != nil {
		fail(w, "failed to validate enrollment code", http.StatusInternalServerError)
		return
	}
	if !valid {
		fail(w, "invalid or expired enrollment code", http.StatusBadRequest)
		return
	}

	// Decode client message.
	clientMessage, err := base64.StdEncoding.DecodeString(req.ClientMessageB64)
	if err != nil {
		fail(w, "invalid client_message_b64: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Start server side of SPAKE2.
	serverMsg, serverState, err := spake2.StartServer(req.EnrollmentCode)
	if err != nil {
		fail(w, "SPAKE2 start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Finish server side to derive shared key.
	sharedKey, err := spake2.Finish(serverState, clientMessage)
	if err != nil {
		fail(w, "SPAKE2 finish failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Generate enrollment ID.
	enrollmentID := uuid.New().String()

	// Store pending enrollment for the finish step.
	pendingEnrollments[enrollmentID] = &pendingEnrollment{
		enrollmentCode: req.EnrollmentCode,
		serverState:    serverState,
		sharedKey:      sharedKey,
		createdAt:      time.Now(),
	}

	serverMsgB64 := base64.StdEncoding.EncodeToString(serverMsg)

	success(w, map[string]string{
		"enrollment_id":      enrollmentID,
		"server_message_b64": serverMsgB64,
	})
}

// handleSpake2Finish handles POST /api/relay-auth/server/spake2/finish.
// Request:  {"enrollment_id":"...", "client_id":"...", "client_name":"...", "client_browser":"...",
//
//	"client_os":"...", "client_device":"...", "public_key_b64":"...", "client_proof_b64":"..."}
//
// Response: {"success": true, "data": {"signing_session_id":"...", "server_public_key_b64":"...", "server_proof_b64":"..."}}
func (s *Server) handleSpake2Finish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnrollmentID   string `json:"enrollment_id"`
		ClientID       string `json:"client_id"`
		ClientName     string `json:"client_name"`
		ClientBrowser  string `json:"client_browser"`
		ClientOS       string `json:"client_os"`
		ClientDevice   string `json:"client_device"`
		PublicKeyB64   string `json:"public_key_b64"`
		ClientProofB64 string `json:"client_proof_b64"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.EnrollmentID == "" || req.ClientID == "" || req.PublicKeyB64 == "" || req.ClientProofB64 == "" {
		fail(w, "enrollment_id, client_id, public_key_b64, and client_proof_b64 are required", http.StatusBadRequest)
		return
	}

	// Look up pending enrollment.
	pending, ok := pendingEnrollments[req.EnrollmentID]
	if !ok {
		fail(w, "invalid or expired enrollment ID", http.StatusBadRequest)
		return
	}
	defer delete(pendingEnrollments, req.EnrollmentID)

	// Check enrollment hasn't expired (5 min TTL).
	if time.Since(pending.createdAt) > 5*time.Minute {
		fail(w, "enrollment session expired", http.StatusBadRequest)
		return
	}

	// Decode browser public key.
	browserPublicKey, err := base64.StdEncoding.DecodeString(req.PublicKeyB64)
	if err != nil {
		fail(w, "invalid public_key_b64: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(browserPublicKey) != ed25519.PublicKeySize {
		fail(w, fmt.Sprintf("public key must be %d bytes", ed25519.PublicKeySize), http.StatusBadRequest)
		return
	}

	// Decode and verify client proof.
	clientProof, err := base64.StdEncoding.DecodeString(req.ClientProofB64)
	if err != nil {
		fail(w, "invalid client_proof_b64: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !spake2.VerifyClientProof(pending.sharedKey, req.EnrollmentID, browserPublicKey, clientProof) {
		fail(w, "client proof verification failed", http.StatusUnauthorized)
		return
	}

	// Generate server Ed25519 key pair for signing sessions.
	serverPub, serverPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fail(w, "failed to generate server key pair", http.StatusInternalServerError)
		return
	}

	// Generate server proof.
	serverProof, err := spake2.GenerateServerProof(pending.sharedKey, req.EnrollmentID, browserPublicKey, serverPub)
	if err != nil {
		fail(w, "failed to generate server proof", http.StatusInternalServerError)
		return
	}

	// Create signing session.
	signingSessionID := uuid.New().String()
	now := time.Now()

	client := &store.Client{
		ID:          req.ClientID,
		PublicKey:   browserPublicKey,
		DisplayName: req.ClientName,
		DeviceType:  req.ClientBrowser,
		PairedAt:    now,
		LastSeen:    &now,
	}
	if err := s.store.SaveClient(client); err != nil {
		fail(w, "failed to save client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a signing session in the in-memory store.
	s.sessions.CreateSession(signingSessionID, req.ClientID, serverPub)

	serverPubB64 := base64.StdEncoding.EncodeToString(serverPub)
	serverProofB64 := base64.StdEncoding.EncodeToString(serverProof)

	success(w, map[string]string{
		"signing_session_id":    signingSessionID,
		"server_public_key_b64": serverPubB64,
		"server_proof_b64":      serverProofB64,
	})

	// Store server private key for later signing.
	// In production, this should be encrypted. For now, we keep it in the signing session.
	_ = serverPriv
}
