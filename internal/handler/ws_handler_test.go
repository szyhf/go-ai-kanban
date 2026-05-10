package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// TestLogMsgToWSMessage verifies LogMsg → WS JSON conversion.
func TestLogMsgToWSMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  service.LogMsg
		want string
		skip bool
	}{
		{
			name: "ready",
			msg:  service.NewReadyLogMsg(),
			want: `{"Ready":true}`,
		},
		{
			name: "finished",
			msg:  service.NewFinishedLogMsg(),
			want: `{"finished":true}`,
		},
		{
			name: "patch",
			msg: service.NewPatchLogMsg(service.PatchOperation{
				Op: "add", Path: "/workspaces/abc", Value: "test",
			}),
			want: `{"JsonPatch":[{"op":"add","path":"/workspaces/abc","value":"test"}]}`,
		},
		{
			name: "stdout skipped",
			msg:  service.NewStdoutLogMsg("hello"),
			skip: true,
		},
		{
			name: "stderr skipped",
			msg:  service.NewStderrLogMsg("error"),
			skip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.msg.ToWSMessage()
			if err != nil {
				t.Fatalf("ToWSMessage error: %v", err)
			}
			if tt.skip {
				if data != nil {
					t.Errorf("expected nil, got %s", data)
				}
				return
			}
			if string(data) != tt.want {
				// Compare as JSON for field order independence.
				var gotParsed, wantParsed interface{}
				json.Unmarshal(data, &gotParsed)
				json.Unmarshal([]byte(tt.want), &wantParsed)
				gotJ, _ := json.Marshal(gotParsed)
				wantJ, _ := json.Marshal(wantParsed)
				if string(gotJ) != string(wantJ) {
					t.Errorf("got %s, want %s", data, tt.want)
				}
			}
		})
	}
}

// TestLogMsgToLogEntryWSMessage verifies stdout/stderr → log entry conversion.
func TestLogMsgToLogEntryWSMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  service.LogMsg
		want string
		skip bool
	}{
		{
			name: "stdout",
			msg:  service.NewStdoutLogMsg("hello world"),
			want: `{"JsonPatch":[{"op":"add","path":"/entries/-","value":{"type":"STDOUT","content":"hello world"}}]}`,
		},
		{
			name: "stderr",
			msg:  service.NewStderrLogMsg("error msg"),
			want: `{"JsonPatch":[{"op":"add","path":"/entries/-","value":{"type":"STDERR","content":"error msg"}}]}`,
		},
		{
			name: "patch skipped",
			msg: service.NewPatchLogMsg(service.PatchOperation{Op: "add", Path: "/x"}),
			skip: true,
		},
		{
			name: "ready skipped",
			msg:  service.NewReadyLogMsg(),
			skip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.msg.ToLogEntryWSMessage()
			if err != nil {
				t.Fatalf("ToLogEntryWSMessage error: %v", err)
			}
			if tt.skip {
				if data != nil {
					t.Errorf("expected nil, got %s", data)
				}
				return
			}
			// Compare as JSON.
			var gotParsed, wantParsed interface{}
			json.Unmarshal(data, &gotParsed)
			json.Unmarshal([]byte(tt.want), &wantParsed)
			gotJ, _ := json.Marshal(gotParsed)
			wantJ, _ := json.Marshal(wantParsed)
			if string(gotJ) != string(wantJ) {
				t.Errorf("got %s, want %s", gotJ, wantJ)
			}
		})
	}
}

// TestHandleWSStream_PatchStream tests the generic WS stream handler with patch messages.
func TestHandleWSStream_PatchStream(t *testing.T) {
	// Create a channel to simulate subscription.
	ch := make(chan service.LogMsg, 10)
	closeCh := make(chan struct{})

	cfg := wsStreamConfig{
		InitialMessages: []service.LogMsg{
			service.NewPatchLogMsg(service.PatchOperation{
				Op: "replace", Path: "/workspaces", Value: []string{},
			}),
		},
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return ch, func() { close(closeCh) }
		},
		Convert: defaultWSConvert,
	}

	// Start a test server.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleWSStream(w, r, cfg)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Read initial snapshot.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snapshot map[string]interface{}
	json.Unmarshal(msg, &snapshot)
	if _, ok := snapshot["JsonPatch"]; !ok {
		t.Errorf("expected JsonPatch in snapshot, got %s", msg)
	}

	// Read Ready signal.
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ready: %v", err)
	}
	if string(msg) != `{"Ready":true}` {
		t.Errorf("expected Ready, got %s", msg)
	}

	// Push a patch.
	ch <- service.NewPatchLogMsg(service.PatchOperation{
		Op: "add", Path: "/workspaces/123", Value: "new-ws",
	})

	// Read the patch.
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read patch: %v", err)
	}
	var patchMsg map[string]interface{}
	json.Unmarshal(msg, &patchMsg)
	if _, ok := patchMsg["JsonPatch"]; !ok {
		t.Errorf("expected JsonPatch, got %s", msg)
	}

	// Close.
	conn.Close()
}

// TestHandleWSStream_LogStream tests the WS stream with log entry conversion.
func TestHandleWSStream_LogStream(t *testing.T) {
	ch := make(chan service.LogMsg, 10)
	closeCh := make(chan struct{})

	cfg := wsStreamConfig{
		InitialMessages: []service.LogMsg{
			service.NewStdoutLogMsg("initial output"),
		},
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return ch, func() { close(closeCh) }
		},
		Convert: logEntryWSConvert,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleWSStream(w, r, cfg)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Read initial log entry.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read initial: %v", err)
	}
	var initial map[string]interface{}
	json.Unmarshal(msg, &initial)
	patches, ok := initial["JsonPatch"].([]interface{})
	if !ok || len(patches) != 1 {
		t.Fatalf("expected JsonPatch array, got %s", msg)
	}
	entry := patches[0].(map[string]interface{})
	value := entry["value"].(map[string]interface{})
	if value["type"] != "STDOUT" {
		t.Errorf("expected STDOUT, got %v", value["type"])
	}
	if value["content"] != "initial output" {
		t.Errorf("expected 'initial output', got %v", value["content"])
	}

	// Read Ready.
	_, msg, _ = conn.ReadMessage()
	if string(msg) != `{"Ready":true}` {
		t.Errorf("expected Ready, got %s", msg)
	}

	conn.Close()
}

// TestWorkspaceStreamWS tests the workspace stream endpoint.
func TestWorkspaceStreamWS(t *testing.T) {
	_, router := setupTestHandler(t)

	// Create a workspace first.
	wsBody := map[string]interface{}{
		"branch": "ws-stream-test",
		"name":   "stream-test-ws",
	}
	rec := doRequest(t, router, http.MethodPost, "/api/workspaces", wsBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create workspace: %d %s", rec.Code, rec.Body.String())
	}

	// Connect to WS.
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/workspaces/streams/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Read snapshot + Ready.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snapshot map[string]interface{}
	json.Unmarshal(msg, &snapshot)
	if _, ok := snapshot["JsonPatch"]; !ok {
		t.Errorf("expected JsonPatch snapshot, got %s", msg)
	}

	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ready: %v", err)
	}
	if string(msg) != `{"Ready":true}` {
		t.Errorf("expected Ready, got %s", msg)
	}

	conn.Close()
}

// TestScratchStreamWS tests the scratch stream endpoint.
func TestScratchStreamWS(t *testing.T) {
	_, router := setupTestHandler(t)

	// Create scratch first.
	scratchID := "00000000-0000-0000-0000-000000000099"
	body := map[string]interface{}{
		"type": "WORKSPACE_NOTES",
		"data": map[string]string{"content": "test notes"},
	}
	rec := doRequest(t, router, http.MethodPut, "/api/scratch/WORKSPACE_NOTES/"+scratchID, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert scratch: %d %s", rec.Code, rec.Body.String())
	}

	// Connect to WS.
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/scratch/WORKSPACE_NOTES/" + scratchID + "/stream/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Read Ready.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// First message is either snapshot or Ready.
	var parsed map[string]interface{}
	json.Unmarshal(msg, &parsed)
	if _, ok := parsed["JsonPatch"]; !ok && string(msg) != `{"Ready":true}` {
		t.Errorf("expected JsonPatch or Ready, got %s", msg)
	}

	conn.Close()
}

// TestApprovalStreamWS tests the approval stream endpoint.
func TestApprovalStreamWS(t *testing.T) {
	_, router := setupTestHandler(t)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/approvals/stream/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Read snapshot.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snapshot map[string]interface{}
	json.Unmarshal(msg, &snapshot)
	if _, ok := snapshot["JsonPatch"]; !ok {
		t.Errorf("expected JsonPatch, got %s", msg)
	}

	// Read Ready.
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ready: %v", err)
	}
	if string(msg) != `{"Ready":true}` {
		t.Errorf("expected Ready, got %s", msg)
	}

	conn.Close()
}

// TestExecProcessStreamWS_MissingSessionID tests that session_id is required.
func TestExecProcessStreamWS_MissingSessionID(t *testing.T) {
	_, router := setupTestHandler(t)

	rec := doRequest(t, router, http.MethodGet, "/api/execution-processes/stream/session/ws", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestRawLogsWS_NotFound tests raw-logs for a non-existent process.
func TestRawLogsWS_NotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/execution-processes/00000000-0000-0000-0000-000000000000/raw-logs/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Should receive finished message immediately.
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != `{"finished":true}` {
		t.Errorf("expected finished, got %s", msg)
	}
}
