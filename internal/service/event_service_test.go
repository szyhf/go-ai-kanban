package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEventService_NotifyWorkspaceInsert(t *testing.T) {
	msgStore := NewMsgStore()

	ch, unsub := msgStore.Subscribe()
	defer unsub()

	// Simulate a workspace insert notification (without actual DB).
	// For unit testing, we directly test patch generation.
	patch := WorkspacePatch("add", "ws-123", map[string]string{"name": "test"})
	msgStore.PushPatch(patch)

	msg := <-ch
	if msg.Kind != LogMsgPatch {
		t.Errorf("expected patch kind, got %q", msg.Kind)
	}
	if msg.Patch.Op != "add" {
		t.Errorf("expected add op, got %q", msg.Patch.Op)
	}
	if !strings.HasPrefix(msg.Patch.Path, "/workspaces/") {
		t.Errorf("expected /workspaces/ prefix, got %q", msg.Patch.Path)
	}
}

func TestEventService_NotifyWorkspaceDelete(t *testing.T) {
	msgStore := NewMsgStore()
	svc := NewEventService(msgStore, nil)

	ch, unsub := msgStore.Subscribe()
	defer unsub()

	svc.NotifyDelete(HookTableWorkspaces, newTestUUID("ws-del"))

	msg := <-ch
	if msg.Patch.Op != "remove" {
		t.Errorf("expected remove op, got %q", msg.Patch.Op)
	}
}

func TestEventService_FilteredSubscribe(t *testing.T) {
	msgStore := NewMsgStore()
	svc := NewEventService(msgStore, nil)

	ch, unsub := svc.FilteredSubscribe("/workspaces")
	defer unsub()

	// Push a matching patch.
	msgStore.PushPatch(WorkspacePatch("add", "ws-1", map[string]string{"name": "test"}))
	// Push a non-matching patch.
	msgStore.PushPatch(ExecutionProcessPatch("add", "ep-1", nil))

	msg := <-ch
	if msg.Patch == nil || !strings.HasPrefix(msg.Patch.Path, "/workspaces") {
		t.Errorf("expected workspace patch, got %+v", msg)
	}
}

func TestWorkspacePatch(t *testing.T) {
	patch := WorkspacePatch("add", "abc-123", map[string]string{"name": "my workspace"})
	if patch.Op != "add" {
		t.Errorf("expected add, got %q", patch.Op)
	}
	if patch.Path != "/workspaces/abc-123" {
		t.Errorf("unexpected path: %q", patch.Path)
	}
}

func TestExecutionProcessPatch(t *testing.T) {
	patch := ExecutionProcessPatch("replace", "ep-456", nil)
	if patch.Op != "replace" {
		t.Errorf("expected replace, got %q", patch.Op)
	}
	if patch.Path != "/execution_processes/ep-456" {
		t.Errorf("unexpected path: %q", patch.Path)
	}
}

func TestScratchPatch(t *testing.T) {
	patch := ScratchPatch("replace", map[string]string{"content": "hello"})
	if patch.Path != "/scratch" {
		t.Errorf("unexpected path: %q", patch.Path)
	}
}

func TestScratchRemovePatch(t *testing.T) {
	patch := ScratchRemovePatch("scratch-1", "note")
	if patch.Op != "replace" {
		t.Errorf("expected replace for scratch remove, got %q", patch.Op)
	}
	if patch.Path != "/scratch" {
		t.Errorf("unexpected path: %q", patch.Path)
	}
}

func TestEscapeJSONPointer(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"abc", "abc"},
		{"a/b", "a~1b"},
		{"a~b", "a~0b"},
		{"a/b~c", "a~1b~0c"},
	}
	for _, tt := range tests {
		got := escapeJSONPointer(tt.input)
		if got != tt.want {
			t.Errorf("escapeJSONPointer(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPatchPathMatches(t *testing.T) {
	patch := PatchOperation{Op: "add", Path: "/workspaces/abc"}
	if !PatchPathMatches(patch, "/workspaces") {
		t.Error("expected match")
	}
	if PatchPathMatches(patch, "/scratch") {
		t.Error("expected no match")
	}
}

func TestPatchPathHasPrefix(t *testing.T) {
	patch := PatchOperation{Op: "add", Path: "/workspaces/abc/def"}
	if !PatchPathHasPrefix(patch, "/workspaces") {
		t.Error("expected match with /workspaces prefix")
	}
	if !PatchPathHasPrefix(patch, "/workspaces/abc") {
		t.Error("expected match with /workspaces/abc prefix")
	}
}

func TestMarshalPatchValue(t *testing.T) {
	raw := MarshalPatchValue(map[string]string{"name": "test"})
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["name"] != "test" {
		t.Errorf("unexpected value: %v", m)
	}
}

func TestIsWorkspacePatch(t *testing.T) {
	msg := LogMsg{Kind: LogMsgPatch, Patch: &PatchOperation{Path: "/workspaces/123"}}
	if !IsWorkspacePatch(msg) {
		t.Error("expected workspace patch")
	}

	msg2 := LogMsg{Kind: LogMsgPatch, Patch: &PatchOperation{Path: "/scratch"}}
	if IsWorkspacePatch(msg2) {
		t.Error("should not match scratch as workspace")
	}
}

// newTestUUID creates a UUID from a string for testing.
func newTestUUID(s string) [16]byte {
	var uuid [16]byte
	copy(uuid[:], s)
	return uuid
}
