package signaling

import (
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"
)

// Handler reads messages from a peer's WebSocket connection and dispatches
// them to the hub for routing. It handles the initial registration message
// and then forwards all subsequent messages.
type Handler struct {
	hub  *Hub
	peer *Peer
}

// NewHandler creates a handler for a new WebSocket connection.
func NewHandler(hub *Hub, conn *websocket.Conn) *Handler {
	return &Handler{
		hub:  hub,
		peer: NewPeer(conn),
	}
}

// Serve reads messages from the peer until the connection closes or the
// peer's context is cancelled.
func (h *Handler) Serve() {
	defer func() {
		if h.peer.ID != "" {
			h.hub.Unregister(h.peer.ID)
		} else {
			h.peer.Close()
		}
	}()

	// First message must be a registration.
	if !h.handleRegister() {
		return
	}

	// Main message loop.
	for {
		select {
		case <-h.peer.Context().Done():
			return
		default:
		}

		data, err := h.peer.ReadMessage()
		if err != nil {
			// Normal closure or context cancelled.
			log.Printf("handler: peer %s read error: %v", h.peer.ID, err)
			return
		}

		env, err := ParseEnvelope(data)
		if err != nil {
			_ = h.peer.SendError("invalid_json", "message must be valid JSON")
			continue
		}

		// Handle heartbeat inline for efficiency.
		if env.Type == TypeHeartbeat {
			_ = h.peer.SendPayload(TypeHeartbeatAck, HeartbeatPayload{
				Timestamp: time.Now().UnixMilli(),
			})
			continue
		}

		// Dispatch to hub for routing.
		h.hub.Inbound() <- &InboundMessage{
			Peer:     h.peer,
			Envelope: env,
		}
	}
}

// handleRegister processes the first registration message.
// Returns false if registration fails (connection will be closed).
func (h *Handler) handleRegister() bool {
	data, err := h.peer.ReadMessage()
	if err != nil {
		log.Printf("handler: failed to read register message: %v", err)
		return false
	}

	env, err := ParseEnvelope(data)
	if err != nil {
		_ = h.peer.SendError("invalid_json", "first message must be valid JSON")
		return false
	}

	if env.Type != TypeRegister {
		_ = h.peer.SendError("expected_register", "first message must be type 'register'")
		return false
	}

	var reg RegisterPayload
	if err := json.Unmarshal(env.Payload, &reg); err != nil {
		_ = h.peer.SendError("invalid_payload", "invalid register payload")
		return false
	}

	if reg.ID == "" {
		_ = h.peer.SendError("missing_id", "register payload must include 'id'")
		return false
	}

	if reg.Role != "client" && reg.Role != "host" {
		_ = h.peer.SendError("invalid_role", "role must be 'client' or 'host'")
		return false
	}

	// Set peer identity.
	h.peer.ID = reg.ID
	h.peer.Role = reg.Role
	if reg.Role == "client" {
		h.peer.ClientID = reg.ID
	} else {
		h.peer.ClientID = reg.ClientID
	}

	// Register host-client relationship.
	if reg.Role == "host" && reg.ClientID != "" {
		h.hub.RegisterHostClient(reg.ID, reg.ClientID)
	}

	// Add to hub.
	h.hub.Register(h.peer)

	// Send acknowledgment.
	_ = h.peer.SendPayload(TypeRegisterAck, RegisterAckPayload{Status: "ok"})

	log.Printf("handler: peer %s registered as %s", reg.ID, reg.Role)
	return true
}
