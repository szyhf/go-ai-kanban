package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProjectJSON(t *testing.T) {
	p := Project{
		ID:        NewUUID(),
		Name:      "Test Project",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// Verify snake_case JSON keys
	expectedKeys := []string{"id", "name", "default_agent_working_dir", "remote_project_id", "created_at", "updated_at"}
	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in JSON: %v", key, string(data))
		}
	}

	// Round-trip
	var p2 Project
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p2.ID != p.ID {
		t.Errorf("ID mismatch")
	}
	if p2.Name != p.Name {
		t.Errorf("Name mismatch")
	}
}

func TestTaskJSON(t *testing.T) {
	task := Task{
		ID:        NewUUID(),
		ProjectID: NewUUID(),
		Title:     "Fix bug",
		Status:    TaskStatusInProgress,
		CreatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify status serializes as lowercase
	if !strings.Contains(string(data), `"status":"inprogress"`) {
		t.Errorf("status should be 'inprogress', got: %s", data)
	}

	// Verify description is null (not omitted)
	if !strings.Contains(string(data), `"description":null`) {
		t.Errorf("description should be null, got: %s", data)
	}

	// Round-trip
	var task2 Task
	if err := json.Unmarshal(data, &task2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if task2.Status != TaskStatusInProgress {
		t.Errorf("Status mismatch: got %s", task2.Status)
	}
}

func TestWorkspaceWithStatusFlatten(t *testing.T) {
	ws := WorkspaceWithStatus{
		Workspace: Workspace{
			ID:     NewUUID(),
			Branch: "feature/test",
		},
		IsRunning: true,
		IsErrored: false,
	}

	data, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// Workspace fields should be at top level (flattened)
	if _, ok := m["branch"]; !ok {
		t.Error("branch should be at top level (flattened)")
	}
	if _, ok := m["is_running"]; !ok {
		t.Error("is_running should be at top level")
	}
	if _, ok := m["is_errored"]; !ok {
		t.Error("is_errored should be at top level")
	}
	// workspace key should NOT exist
	if _, ok := m["workspace"]; ok {
		t.Error("workspace should NOT be a separate key (should be flattened)")
	}
}

func TestRepoWithTargetBranchFlatten(t *testing.T) {
	r := RepoWithTargetBranch{
		Repo: Repo{
			ID:          NewUUID(),
			Path:        "/path/to/repo",
			Name:        "myrepo",
			DisplayName: "My Repo",
		},
		TargetBranch: "main",
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// Repo fields should be at top level
	if _, ok := m["path"]; !ok {
		t.Error("path should be at top level")
	}
	if _, ok := m["target_branch"]; !ok {
		t.Error("target_branch should be at top level")
	}
	if _, ok := m["repo"]; ok {
		t.Error("repo should NOT be a separate key")
	}
}

func TestMergeDirectJSON(t *testing.T) {
	m := NewDirectMerge(DirectMerge{
		ID:               NewUUID(),
		WorkspaceID:      NewUUID(),
		RepoID:           NewUUID(),
		MergeCommit:      "abc123",
		TargetBranchName: "main",
		CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Should have type: "direct"
	if !strings.Contains(string(data), `"type":"direct"`) {
		t.Errorf("type should be 'direct', got: %s", data)
	}
	// Should have merge_commit
	if !strings.Contains(string(data), `"merge_commit":"abc123"`) {
		t.Errorf("should have merge_commit, got: %s", data)
	}
	// Should NOT have pr_info
	if strings.Contains(string(data), "pr_info") {
		t.Errorf("should NOT have pr_info, got: %s", data)
	}

	// Round-trip
	var m2 Merge
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m2.Type != MergeTypeDirect {
		t.Errorf("type mismatch: got %s", m2.Type)
	}
	d := m2.AsDirect()
	if d == nil {
		t.Fatal("AsDirect should not be nil")
	}
	if d.MergeCommit != "abc123" {
		t.Errorf("MergeCommit mismatch: got %s", d.MergeCommit)
	}
}

func TestMergePrJSON(t *testing.T) {
	m := NewPrMerge(PrMerge{
		ID:               NewUUID(),
		WorkspaceID:      NewUUID(),
		RepoID:           NewUUID(),
		CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		TargetBranchName: "main",
		PrInfo: PullRequestInfo{
			Number: 42,
			URL:    "https://github.com/org/repo/pull/42",
			Status: MergeStatusOpen,
		},
	})

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Should have type: "pr"
	if !strings.Contains(string(data), `"type":"pr"`) {
		t.Errorf("type should be 'pr', got: %s", data)
	}
	// Should have pr_info
	if !strings.Contains(string(data), `"pr_info"`) {
		t.Errorf("should have pr_info, got: %s", data)
	}
	// Should NOT have merge_commit
	if strings.Contains(string(data), `"merge_commit"`) {
		t.Errorf("should NOT have merge_commit, got: %s", data)
	}

	// Round-trip
	var m2 Merge
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := m2.AsPr()
	if p == nil {
		t.Fatal("AsPr should not be nil")
	}
	if p.PrInfo.Number != 42 {
		t.Errorf("PR number mismatch: got %d", p.PrInfo.Number)
	}
}

func TestScratchPayloadDraftTask(t *testing.T) {
	sp := ScratchPayload{
		Type: ScratchTypeDraftTask,
		Data: []byte(`"do something"`),
	}

	data, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Adjacently tagged: {"type": "DRAFT_TASK", "data": "do something"}
	if !strings.Contains(string(data), `"type":"DRAFT_TASK"`) {
		t.Errorf("type mismatch, got: %s", data)
	}
	if !strings.Contains(string(data), `"data":"do something"`) {
		t.Errorf("data mismatch, got: %s", data)
	}

	// Accessor
	s, err := sp.AsDraftTask()
	if err != nil {
		t.Fatalf("AsDraftTask: %v", err)
	}
	if s != "do something" {
		t.Errorf("got %q, want 'do something'", s)
	}
}

func TestScratchPayloadDraftFollowUp(t *testing.T) {
	sp := ScratchPayload{
		Type: ScratchTypeDraftFollowUp,
		Data: []byte(`{"message":"fix the bug","executor_config":{}}`),
	}

	d, err := sp.AsDraftFollowUp()
	if err != nil {
		t.Fatalf("AsDraftFollowUp: %v", err)
	}
	if d.Message != "fix the bug" {
		t.Errorf("got %q, want 'fix the bug'", d.Message)
	}
}

func TestScratchPayloadWrongType(t *testing.T) {
	sp := ScratchPayload{
		Type: ScratchTypeDraftTask,
		Data: []byte(`"hello"`),
	}

	_, err := sp.AsDraftFollowUp()
	if err == nil {
		t.Error("expected error for wrong type accessor")
	}
}

func TestScratchFullRoundTrip(t *testing.T) {
	scratch := Scratch{
		ID: NewUUID(),
		Payload: ScratchPayload{
			Type: ScratchTypeWorkspaceNotes,
			Data: []byte(`{"content":"my notes"}`),
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(scratch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var s2 Scratch
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s2.ID != scratch.ID {
		t.Error("ID mismatch")
	}
	if s2.Payload.Type != ScratchTypeWorkspaceNotes {
		t.Errorf("type mismatch: got %s", s2.Payload.Type)
	}
}
