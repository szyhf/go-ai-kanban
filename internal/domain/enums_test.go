package domain

import (
	"encoding/json"
	"testing"
)

func TestTaskStatusJSON(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskStatusTodo, `"todo"`},
		{TaskStatusInProgress, `"inprogress"`},
		{TaskStatusInReview, `"inreview"`},
		{TaskStatusDone, `"done"`},
		{TaskStatusCancelled, `"cancelled"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.status)
		if err != nil {
			t.Fatalf("marshal %s: %v", tt.status, err)
		}
		if string(data) != tt.expected {
			t.Errorf("TaskStatus %s: got %s, want %s", tt.status, data, tt.expected)
		}

		// Round-trip
		var parsed TaskStatus
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal %s: %v", tt.status, err)
		}
		if parsed != tt.status {
			t.Errorf("round-trip: got %s, want %s", parsed, tt.status)
		}
	}
}

func TestExecStatusJSON(t *testing.T) {
	tests := []struct {
		status   ExecStatus
		expected string
	}{
		{ExecStatusRunning, `"running"`},
		{ExecStatusCompleted, `"completed"`},
		{ExecStatusFailed, `"failed"`},
		{ExecStatusKilled, `"killed"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.status)
		if err != nil {
			t.Fatalf("marshal %s: %v", tt.status, err)
		}
		if string(data) != tt.expected {
			t.Errorf("ExecStatus %s: got %s, want %s", tt.status, data, tt.expected)
		}
	}
}

func TestRunReasonJSON(t *testing.T) {
	tests := []struct {
		reason   RunReason
		expected string
	}{
		{RunReasonSetupScript, `"setupscript"`},
		{RunReasonCleanupScript, `"cleanupscript"`},
		{RunReasonArchiveScript, `"archivescript"`},
		{RunReasonCodingAgent, `"codingagent"`},
		{RunReasonDevServer, `"devserver"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.reason)
		if err != nil {
			t.Fatalf("marshal %s: %v", tt.reason, err)
		}
		if string(data) != tt.expected {
			t.Errorf("RunReason %s: got %s, want %s", tt.reason, data, tt.expected)
		}
	}
}

func TestMergeStatusJSON(t *testing.T) {
	tests := []struct {
		status   MergeStatus
		expected string
	}{
		{MergeStatusOpen, `"open"`},
		{MergeStatusMerged, `"merged"`},
		{MergeStatusClosed, `"closed"`},
		{MergeStatusUnknown, `"unknown"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.status)
		if err != nil {
			t.Fatalf("marshal %s: %v", tt.status, err)
		}
		if string(data) != tt.expected {
			t.Errorf("MergeStatus %s: got %s, want %s", tt.status, data, tt.expected)
		}
	}
}

func TestScratchTypeJSON(t *testing.T) {
	tests := []struct {
		sType    ScratchType
		expected string
	}{
		{ScratchTypeDraftTask, `"DRAFT_TASK"`},
		{ScratchTypeDraftFollowUp, `"DRAFT_FOLLOW_UP"`},
		{ScratchTypeDraftWorkspace, `"DRAFT_WORKSPACE"`},
		{ScratchTypeDraftIssue, `"DRAFT_ISSUE"`},
		{ScratchTypePreviewSettings, `"PREVIEW_SETTINGS"`},
		{ScratchTypeWorkspaceNotes, `"WORKSPACE_NOTES"`},
		{ScratchTypeUIPreferences, `"UI_PREFERENCES"`},
		{ScratchTypeProjectRepoDefaults, `"PROJECT_REPO_DEFAULTS"`},
	}

	for _, tt := range tests {
		data, err := json.Marshal(tt.sType)
		if err != nil {
			t.Fatalf("marshal %s: %v", tt.sType, err)
		}
		if string(data) != tt.expected {
			t.Errorf("ScratchType %s: got %s, want %s", tt.sType, data, tt.expected)
		}
	}
}

func TestEnumScanValue(t *testing.T) {
	// Test TaskStatus Scan with string
	var ts TaskStatus
	if err := ts.Scan("todo"); err != nil {
		t.Fatalf("TaskStatus.Scan string: %v", err)
	}
	if ts != TaskStatusTodo {
		t.Errorf("got %s, want %s", ts, TaskStatusTodo)
	}

	// Test TaskStatus Scan with []byte
	if err := ts.Scan([]byte("done")); err != nil {
		t.Fatalf("TaskStatus.Scan []byte: %v", err)
	}
	if ts != TaskStatusDone {
		t.Errorf("got %s, want %s", ts, TaskStatusDone)
	}

	// Test TaskStatus Scan nil -> default
	if err := ts.Scan(nil); err != nil {
		t.Fatalf("TaskStatus.Scan nil: %v", err)
	}
	if ts != TaskStatusTodo {
		t.Errorf("nil should default to todo, got %s", ts)
	}

	// Test Value
	val, err := TaskStatusDone.Value()
	if err != nil {
		t.Fatalf("TaskStatus.Value: %v", err)
	}
	if val != "done" {
		t.Errorf("got %v, want 'done'", val)
	}
}
