package signaling

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub maintains the set of active WebSocket peers and routes messages
// between paired clients and hosts.
type Hub struct {
	mu    sync.RWMutex
	peers map[string]*Peer // keyed by peer ID (client ID or host ID)

	// clientHosts maps clientID → set of hostIDs that belong to this client.
	clientHosts map[string]map[string]struct{}

	// hostClient maps hostID → clientID (the client that owns the host).
	hostClient map[string]string

	// Channels for peer lifecycle.
	register   chan *Peer
	unregister chan string // peer ID

	// Incoming messages from peers (populated by handler).
	inbound chan *InboundMessage
}

// InboundMessage wraps a message received from a peer.
type InboundMessage struct {
	Peer     *Peer
	Envelope *Envelope
}

// NewHub creates a new Hub ready to accept connections.
func NewHub() *Hub {
	return &Hub{
		peers:       make(map[string]*Peer),
		clientHosts: make(map[string]map[string]struct{}),
		hostClient:  make(map[string]string),
		register:    make(chan *Peer, 64),
		unregister:  make(chan string, 64),
		inbound:     make(chan *InboundMessage, 256),
	}
}

// Register adds a peer to the hub.
func (h *Hub) Register(peer *Peer) {
	h.register <- peer
}

// Unregister removes a peer from the hub.
func (h *Hub) Unregister(peerID string) {
	h.unregister <- peerID
}

// Inbound returns the channel for incoming messages.
func (h *Hub) Inbound() chan<- *InboundMessage {
	return h.inbound
}

// Run starts the hub's event loop. Blocks until Stop is called.
func (h *Hub) Run(stop <-chan struct{}) {
	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-stop:
			h.closeAll()
			return

		case peer := <-h.register:
			h.mu.Lock()
			h.peers[peer.ID] = peer
			h.mu.Unlock()
			log.Printf("hub: peer registered %s (%s)", peer.ID, peer.Role)

		case id := <-h.unregister:
			h.mu.Lock()
			if peer, ok := h.peers[id]; ok {
				peer.Close()
				delete(h.peers, id)
				// Clean up host-client mappings.
				if peer.Role == "host" {
					if clientID, ok := h.hostClient[id]; ok {
						delete(h.hostClient, id)
						if hosts, ok := h.clientHosts[clientID]; ok {
							delete(hosts, id)
						}
					}
				}
				log.Printf("hub: peer unregistered %s (%s)", id, peer.Role)
			}
			h.mu.Unlock()

		case msg := <-h.inbound:
			h.route(msg)

		case <-cleanupTicker.C:
			h.cleanupIdle()
		}
	}
}

// route dispatches an inbound message based on its type.
func (h *Hub) route(msg *InboundMessage) {
	switch msg.Envelope.Type {
	case TypeHeartbeat:
		_ = msg.Peer.SendPayload(TypeHeartbeatAck, HeartbeatPayload{Timestamp: time.Now().UnixMilli()})
	case TypeProxyRequest, TypeProxyResponse:
		h.routeProxyMessage(msg)
	case TypeSDPOffer, TypeSDPAnswer, TypeICECandidate:
		h.routeWebRTCMessage(msg)
	default:
		_ = msg.Peer.SendError("unknown_type", "unknown message type: "+string(msg.Envelope.Type))
	}
}

// routeProxyMessage forwards a proxy request/response to the appropriate peer.
func (h *Hub) routeProxyMessage(msg *InboundMessage) {
	var targetID string

	switch msg.Envelope.Type {
	case TypeProxyRequest:
		// Client sends proxy_request → forward to host.
		var payload ProxyRequestPayload
		if err := json.Unmarshal(msg.Envelope.Payload, &payload); err != nil {
			_ = msg.Peer.SendError("invalid_payload", "invalid proxy_request payload")
			return
		}
		// Track request so the response can be routed back.
		h.trackRequest(payload.RequestID, msg.Peer.ID)
		// The path encodes the target host: /h/{hostId}/...
		targetID = extractHostIDFromPath(payload.Path)
		if targetID == "" {
			_ = msg.Peer.SendError("invalid_path", "proxy path must include host ID")
			return
		}

	case TypeProxyResponse:
		// Host sends proxy_response → forward to client.
		var payload ProxyResponsePayload
		if err := json.Unmarshal(msg.Envelope.Payload, &payload); err != nil {
			_ = msg.Peer.SendError("invalid_payload", "invalid proxy_response payload")
			return
		}
		// We need to determine the target client. The request ID is used to
		// look up the originating client (stored in a pending requests map).
		targetID = h.findClientForRequest(payload.RequestID)
		if targetID == "" {
			_ = msg.Peer.SendError("unknown_request", "no pending request found for "+payload.RequestID)
			return
		}
	}

	h.forwardToPeer(targetID, msg)
}

// routeWebRTCMessage forwards WebRTC SDP/ICE messages to the target peer.
func (h *Hub) routeWebRTCMessage(msg *InboundMessage) {
	var targetID string

	switch msg.Envelope.Type {
	case TypeSDPOffer, TypeSDPAnswer:
		var payload SDPPayload
		if err := json.Unmarshal(msg.Envelope.Payload, &payload); err != nil {
			_ = msg.Peer.SendError("invalid_payload", "invalid SDP payload")
			return
		}
		targetID = payload.TargetID
	case TypeICECandidate:
		var payload ICECandidatePayload
		if err := json.Unmarshal(msg.Envelope.Payload, &payload); err != nil {
			_ = msg.Peer.SendError("invalid_payload", "invalid ICE candidate payload")
			return
		}
		targetID = payload.TargetID
	}

	h.forwardToPeer(targetID, msg)
}

// forwardToPeer sends a message envelope to the specified peer.
func (h *Hub) forwardToPeer(targetID string, msg *InboundMessage) {
	h.mu.RLock()
	peer, ok := h.peers[targetID]
	h.mu.RUnlock()

	if !ok {
		_ = msg.Peer.SendError("peer_not_found", "target peer not found: "+targetID)
		return
	}
	if err := peer.Send(msg.Envelope); err != nil {
		log.Printf("hub: failed to forward to %s: %v", targetID, err)
		h.Unregister(targetID)
	}
}

// closeAll closes all peer connections.
func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, peer := range h.peers {
		peer.Close()
		delete(h.peers, id)
	}
}

// cleanupIdle removes peers with cancelled contexts.
func (h *Hub) cleanupIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, peer := range h.peers {
		select {
		case <-peer.Context().Done():
			peer.Close()
			delete(h.peers, id)
			log.Printf("hub: cleaned up idle peer %s", id)
		default:
		}
	}
}

// GetPeer returns a peer by ID.
func (h *Hub) GetPeer(id string) (*Peer, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	p, ok := h.peers[id]
	return p, ok
}

// PeerCount returns the number of connected peers.
func (h *Hub) PeerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.peers)
}

// RegisterHostClient associates a host with its owning client.
func (h *Hub) RegisterHostClient(hostID, clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hostClient[hostID] = clientID
	if h.clientHosts[clientID] == nil {
		h.clientHosts[clientID] = make(map[string]struct{})
	}
	h.clientHosts[clientID][hostID] = struct{}{}
}

// pendingRequests tracks proxy_request → originating client ID.
// This is needed to route proxy_response back to the correct client.
var (
	pendingMu       sync.RWMutex
	pendingRequests = make(map[string]string) // requestID → clientID
)

func (h *Hub) trackRequest(requestID, clientID string) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingRequests[requestID] = clientID
}

func (h *Hub) findClientForRequest(requestID string) string {
	pendingMu.RLock()
	defer pendingMu.RUnlock()
	return pendingRequests[requestID]
}

// extractHostIDFromPath extracts the host ID from a proxy path like "/h/{hostId}/...".
func extractHostIDFromPath(path string) string {
	// Expected format: /h/{hostId}/... or /v1/relay/h/{hostId}/s/{sessionId}/...
	parts := splitPath(path)
	for i, p := range parts {
		if p == "h" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// splitPath splits a URL path into non-empty segments.
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	return parts
}
