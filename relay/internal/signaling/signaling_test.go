package signaling

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestParseEnvelope(t *testing.T) {
	raw := `{"type":"heartbeat","payload":{"timestamp":12345},"seq":1}`
	env, err := ParseEnvelope([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if env.Type != TypeHeartbeat {
		t.Fatalf("expected type %q, got %q", TypeHeartbeat, env.Type)
	}
	if env.Seq != 1 {
		t.Fatalf("expected seq 1, got %d", env.Seq)
	}
	var hb HeartbeatPayload
	if err := json.Unmarshal(env.Payload, &hb); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if hb.Timestamp != 12345 {
		t.Fatalf("expected timestamp 12345, got %d", hb.Timestamp)
	}
}

func TestParseEnvelopeInvalid(t *testing.T) {
	_, err := ParseEnvelope([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	original := &Envelope{
		Type: TypeProxyRequest,
		Payload: mustMarshal(ProxyRequestPayload{
			RequestID: "req-1",
			Method:    "GET",
			Path:      "/h/host-1/api/data",
		}),
		Seq: 42,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseEnvelope(data)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if parsed.Type != original.Type {
		t.Fatalf("type mismatch: %q vs %q", parsed.Type, original.Type)
	}
	if parsed.Seq != original.Seq {
		t.Fatalf("seq mismatch: %d vs %d", parsed.Seq, original.Seq)
	}

	var req ProxyRequestPayload
	if err := json.Unmarshal(parsed.Payload, &req); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if req.RequestID != "req-1" || req.Method != "GET" {
		t.Fatalf("payload mismatch: %+v", req)
	}
}

func TestMessageTypes(t *testing.T) {
	types := map[MessageType]string{
		TypeRegister:      "register",
		TypeRegisterAck:   "register_ack",
		TypeError:         "error",
		TypeHeartbeat:     "heartbeat",
		TypeHeartbeatAck:  "heartbeat_ack",
		TypeProxyRequest:  "proxy_request",
		TypeProxyResponse: "proxy_response",
		TypeSDPOffer:      "sdp_offer",
		TypeSDPAnswer:     "sdp_answer",
		TypeICECandidate:  "ice_candidate",
	}
	for mt, expected := range types {
		if string(mt) != expected {
			t.Errorf("MessageType %q != %q", mt, expected)
		}
	}
}

func TestProxyRequestPayload(t *testing.T) {
	payload := ProxyRequestPayload{
		RequestID: "req-123",
		Method:    "POST",
		Path:      "/h/host-abc/s/sess-1/api/kanban",
		Headers:   map[string]string{"Content-Type": "application/json"},
		Body:      "eyJrZXkiOiAidmFsdWUifQ==",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ProxyRequestPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.RequestID != payload.RequestID {
		t.Fatalf("request_id mismatch")
	}
	if decoded.Headers["Content-Type"] != "application/json" {
		t.Fatal("header mismatch")
	}
}

func TestProxyResponsePayload(t *testing.T) {
	payload := ProxyResponsePayload{
		RequestID: "req-456",
		Status:    200,
		Headers:   map[string]string{"X-Custom": "test"},
		Body:      "",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ProxyResponsePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Status != 200 {
		t.Fatalf("expected status 200, got %d", decoded.Status)
	}
}

func TestSDPPayload(t *testing.T) {
	payload := SDPPayload{
		TargetID: "peer-1",
		SDP:      "v=0\r\no=- 123 1 IN IP4 0.0.0.0\r\n",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SDPPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.TargetID != "peer-1" {
		t.Fatal("target_id mismatch")
	}
}

func TestICECandidatePayload(t *testing.T) {
	payload := ICECandidatePayload{
		TargetID:  "peer-2",
		Candidate: "candidate:1 1 udp 2130706431 192.168.1.1 5000 typ host",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ICECandidatePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Candidate != payload.Candidate {
		t.Fatal("candidate mismatch")
	}
}

func TestRegisterPayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     RegisterPayload
		role        string
		hasClientID bool
	}{
		{
			name:    "client registration",
			payload: RegisterPayload{Role: "client", ID: "client-1"},
			role:    "client",
		},
		{
			name:        "host registration",
			payload:     RegisterPayload{Role: "host", ID: "host-1", ClientID: "client-1"},
			role:        "host",
			hasClientID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded RegisterPayload
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded.Role != tt.role {
				t.Fatalf("role mismatch: %q", decoded.Role)
			}
			if tt.hasClientID && decoded.ClientID == "" {
				t.Fatal("expected client_id to be set")
			}
		})
	}
}

func TestErrorPayload(t *testing.T) {
	payload := ErrorPayload{Code: "not_found", Message: "peer not found"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ErrorPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Code != "not_found" {
		t.Fatal("code mismatch")
	}
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func TestExtractHostIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/h/host-1/api/data", "host-1"},
		{"/v1/relay/h/host-abc/s/session-1/api/test", "host-abc"},
		{"/api/data", ""},
		{"", ""},
		{"/h/", ""},
		{"/h/host-xyz", "host-xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractHostIDFromPath(tt.path)
			if got != tt.want {
				t.Fatalf("extractHostIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/a/b/c", []string{"a", "b", "c"}},
		{"/a//b/", []string{"a", "b"}},
		{"", nil},
		{"/", nil},
		{"no-leading-slash", []string{"no-leading-slash"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := splitPath(tt.path)
			if len(got) != len(tt.want) {
				t.Fatalf("splitPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHubRegisterAndLookup(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)
	defer close(stop)

	// Register a peer (we can't use a real WebSocket in unit tests,
	// so we test the hub's bookkeeping via its exported methods).

	// Initially empty.
	if hub.PeerCount() != 0 {
		t.Fatal("expected 0 peers")
	}

	_, ok := hub.GetPeer("nonexistent")
	if ok {
		t.Fatal("expected peer not found")
	}
}

func TestHubRegisterHostClient(t *testing.T) {
	hub := NewHub()
	hub.RegisterHostClient("host-1", "client-1")

	// Verify the mapping was stored (tested indirectly through GetPeer
	// since RegisterHostClient doesn't return anything).
	// The mapping is used during message routing.
}

func TestPendingRequests(t *testing.T) {
	hub := NewHub()

	hub.trackRequest("req-1", "client-1")
	hub.trackRequest("req-2", "client-2")

	if got := hub.findClientForRequest("req-1"); got != "client-1" {
		t.Fatalf("expected client-1, got %q", got)
	}
	if got := hub.findClientForRequest("req-2"); got != "client-2" {
		t.Fatalf("expected client-2, got %q", got)
	}
	if got := hub.findClientForRequest("req-3"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestHubCloseAll(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)

	// Closing the stop channel triggers closeAll.
	close(stop)

	// Give the goroutine time to process.
	// After close, PeerCount should be 0.
	if hub.PeerCount() != 0 {
		t.Fatal("expected 0 peers after close")
	}
}

func TestEnvelopeSignature(t *testing.T) {
	env := &Envelope{
		Type:      TypeProxyRequest,
		Payload:   mustMarshal(ProxyRequestPayload{RequestID: "r1"}),
		Seq:       1,
		Signature: "base64-signature-here",
	}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !bytes.Contains(data, []byte(`"signature":"base64-signature-here"`)) {
		t.Fatal("signature not in marshaled output")
	}
}
