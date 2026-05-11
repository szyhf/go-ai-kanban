package signaling

import "encoding/json"

// MessageType enumerates all WebSocket message types exchanged between
// clients, hosts, and the relay server.
type MessageType string

const (
	// Control messages.
	TypeRegister     MessageType = "register"
	TypeRegisterAck  MessageType = "register_ack"
	TypeError        MessageType = "error"
	TypeHeartbeat    MessageType = "heartbeat"
	TypeHeartbeatAck MessageType = "heartbeat_ack"

	// Proxy messages (client ↔ host via relay).
	TypeProxyRequest  MessageType = "proxy_request"
	TypeProxyResponse MessageType = "proxy_response"

	// WebSocket frame signing (phase 2).
	TypeSigningSessionCreate  MessageType = "signing_session_create"
	TypeSigningSessionRefresh MessageType = "signing_session_refresh"

	// WebRTC signaling (phase 3).
	TypeSDPOffer     MessageType = "sdp_offer"
	TypeSDPAnswer    MessageType = "sdp_answer"
	TypeICECandidate MessageType = "ice_candidate"
)

// Envelope is the top-level JSON envelope for all WebSocket messages.
type Envelope struct {
	Type      MessageType     `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Seq       int64           `json:"seq,omitempty"`
	Signature string          `json:"signature,omitempty"`
}

// RegisterPayload is sent by a client or host immediately after WS upgrade
// to associate the connection with an identity.
type RegisterPayload struct {
	Role     string `json:"role"`                // "client" or "host"
	ID       string `json:"id"`                  // client ID or host ID
	ClientID string `json:"client_id,omitempty"` // for hosts: the owning client ID
}

// RegisterAckPayload is the server's response to a successful registration.
type RegisterAckPayload struct {
	Status string `json:"status"` // "ok"
}

// ErrorPayload carries error details.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HeartbeatPayload carries a timestamp for keep-alive.
type HeartbeatPayload struct {
	Timestamp int64 `json:"timestamp"`
}

// ProxyRequestPayload represents an HTTP request being proxied through the relay.
type ProxyRequestPayload struct {
	RequestID string            `json:"request_id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"` // base64-encoded
}

// ProxyResponsePayload carries the host's HTTP response back through the relay.
type ProxyResponsePayload struct {
	RequestID string            `json:"request_id"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"` // base64-encoded
}

// SDPPayload carries a WebRTC SDP offer or answer.
type SDPPayload struct {
	TargetID string `json:"target_id"` // peer's client/host ID
	SDP      string `json:"sdp"`
}

// ICECandidatePayload carries a WebRTC ICE candidate.
type ICECandidatePayload struct {
	TargetID  string `json:"target_id"`
	Candidate string `json:"candidate"`
}

// ParseEnvelope parses raw JSON bytes into an Envelope.
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
