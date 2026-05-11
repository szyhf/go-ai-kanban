package proxy

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestProxyRequestRoundTrip(t *testing.T) {
	req := &ProxyRequest{
		Method:  "POST",
		Path:    "/api/kanban/tasks",
		Headers: map[string]string{"Authorization": "Bearer token"},
		Body:    []byte(`{"title":"test"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ProxyRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Method != "POST" {
		t.Fatalf("method mismatch: %q", decoded.Method)
	}
	if decoded.Headers["Authorization"] != "Bearer token" {
		t.Fatal("header mismatch")
	}
	if string(decoded.Body) != `{"title":"test"}` {
		t.Fatalf("body mismatch: %q", decoded.Body)
	}
}

func TestProxyResponseRoundTrip(t *testing.T) {
	resp := &ProxyResponse{
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"id":"task-1"}`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ProxyResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.StatusCode != 201 {
		t.Fatalf("status code mismatch: %d", decoded.StatusCode)
	}
}

func TestWriteAndReadRequest(t *testing.T) {
	var buf bytes.Buffer

	req := &ProxyRequest{
		Method: "GET",
		Path:   "/api/data",
		Body:   []byte("hello"),
	}

	if err := WriteRequest(&buf, req); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	decoded, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if decoded.Method != "GET" {
		t.Fatalf("method mismatch: %q", decoded.Method)
	}
	if decoded.Path != "/api/data" {
		t.Fatalf("path mismatch: %q", decoded.Path)
	}
	if string(decoded.Body) != "hello" {
		t.Fatalf("body mismatch: %q", decoded.Body)
	}
}

func TestWriteAndReadResponse(t *testing.T) {
	var buf bytes.Buffer

	resp := &ProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"X-Test": "yes"},
		Body:       []byte("ok"),
	}

	if err := WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}

	decoded, err := ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if decoded.StatusCode != 200 {
		t.Fatalf("status code mismatch: %d", decoded.StatusCode)
	}
	if decoded.Headers["X-Test"] != "yes" {
		t.Fatal("header mismatch")
	}
}

func TestWriteReadRequestEmptyBody(t *testing.T) {
	var buf bytes.Buffer

	req := &ProxyRequest{
		Method: "DELETE",
		Path:   "/api/item/1",
	}

	if err := WriteRequest(&buf, req); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	decoded, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if decoded.Method != "DELETE" {
		t.Fatalf("method mismatch: %q", decoded.Method)
	}
	if len(decoded.Body) != 0 {
		t.Fatalf("expected empty body, got %q", decoded.Body)
	}
}

func TestWebRTCConfig(t *testing.T) {
	cfg := DefaultWebRTCConfig()
	if len(cfg.STUNServers) == 0 {
		t.Fatal("expected at least one STUN server")
	}
	if cfg.STUNServers[0] != "stun:stun.l.google.com:19302" {
		t.Fatalf("unexpected STUN server: %q", cfg.STUNServers[0])
	}
}

func TestDataChannelMessage(t *testing.T) {
	msg := DataChannelMessage{
		Type:    "proxy_request",
		Payload: "base64data",
		Seq:     42,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DataChannelMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "proxy_request" || decoded.Seq != 42 {
		t.Fatalf("mismatch: %+v", decoded)
	}
}

func TestSDPMessage(t *testing.T) {
	msg := SDPMessage{
		Type: "offer",
		SDP:  "v=0\r\no=- 123 1 IN IP4 0.0.0.0\r\ns=-\r\n",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SDPMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "offer" {
		t.Fatalf("type mismatch: %q", decoded.Type)
	}
}

func TestICEMessage(t *testing.T) {
	msg := ICEMessage{
		Candidate:     "candidate:1 1 udp 2130706431 192.168.1.1 5000 typ host",
		SDPMid:        "0",
		SDPMLineIndex: nil,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ICEMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SDPMid != "0" {
		t.Fatalf("sdpMid mismatch: %q", decoded.SDPMid)
	}
}

func TestRelayChannelName(t *testing.T) {
	if RelayChannelName != "relay" {
		t.Fatalf("expected 'relay', got %q", RelayChannelName)
	}
}
