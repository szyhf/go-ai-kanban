package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// TaskStatus represents the status of a task.
// Matches Rust serde rename_all = "lowercase".
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "inprogress"
	TaskStatusInReview   TaskStatus = "inreview"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

func (s TaskStatus) String() string { return string(s) }

// Scan implements sql.Scanner.
func (s *TaskStatus) Scan(value any) error {
	if value == nil {
		*s = TaskStatusTodo
		return nil
	}
	v, ok := value.(string)
	if !ok {
		b, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("TaskStatus: expected string, got %T", value)
		}
		v = string(b)
	}
	*s = TaskStatus(v)
	return nil
}

// Value implements driver.Valuer.
func (s TaskStatus) Value() (driver.Value, error) { return string(s), nil }

// MarshalJSON implements json.Marshaler.
func (s TaskStatus) MarshalJSON() ([]byte, error) { return json.Marshal(string(s)) }

// UnmarshalJSON implements json.Unmarshaler.
func (s *TaskStatus) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = TaskStatus(v)
	return nil
}

// ExecStatus represents the status of an execution process.
// Matches Rust serde rename_all = "lowercase".
type ExecStatus string

const (
	ExecStatusRunning   ExecStatus = "running"
	ExecStatusCompleted ExecStatus = "completed"
	ExecStatusFailed    ExecStatus = "failed"
	ExecStatusKilled    ExecStatus = "killed"
)

func (s ExecStatus) String() string { return string(s) }

// Scan implements sql.Scanner.
func (s *ExecStatus) Scan(value any) error {
	if value == nil {
		*s = ExecStatusRunning
		return nil
	}
	v, ok := value.(string)
	if !ok {
		b, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("ExecStatus: expected string, got %T", value)
		}
		v = string(b)
	}
	*s = ExecStatus(v)
	return nil
}

// Value implements driver.Valuer.
func (s ExecStatus) Value() (driver.Value, error) { return string(s), nil }

// MarshalJSON implements json.Marshaler.
func (s ExecStatus) MarshalJSON() ([]byte, error) { return json.Marshal(string(s)) }

// UnmarshalJSON implements json.Unmarshaler.
func (s *ExecStatus) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = ExecStatus(v)
	return nil
}

// RunReason represents why an execution process was started.
// Matches Rust serde rename_all = "lowercase".
type RunReason string

const (
	RunReasonSetupScript   RunReason = "setupscript"
	RunReasonCleanupScript RunReason = "cleanupscript"
	RunReasonArchiveScript RunReason = "archivescript"
	RunReasonCodingAgent   RunReason = "codingagent"
	RunReasonDevServer     RunReason = "devserver"
)

func (r RunReason) String() string { return string(r) }

// Scan implements sql.Scanner.
func (r *RunReason) Scan(value any) error {
	if value == nil {
		*r = RunReasonSetupScript
		return nil
	}
	v, ok := value.(string)
	if !ok {
		b, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("RunReason: expected string, got %T", value)
		}
		v = string(b)
	}
	*r = RunReason(v)
	return nil
}

// Value implements driver.Valuer.
func (r RunReason) Value() (driver.Value, error) { return string(r), nil }

// MarshalJSON implements json.Marshaler.
func (r RunReason) MarshalJSON() ([]byte, error) { return json.Marshal(string(r)) }

// UnmarshalJSON implements json.Unmarshaler.
func (r *RunReason) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = RunReason(v)
	return nil
}

// MergeStatus represents the status of a merge or pull request.
// Matches Rust serde rename_all = "snake_case".
type MergeStatus string

const (
	MergeStatusOpen    MergeStatus = "open"
	MergeStatusMerged  MergeStatus = "merged"
	MergeStatusClosed  MergeStatus = "closed"
	MergeStatusUnknown MergeStatus = "unknown"
)

func (s MergeStatus) String() string { return string(s) }

// Scan implements sql.Scanner.
func (s *MergeStatus) Scan(value any) error {
	if value == nil {
		*s = MergeStatusUnknown
		return nil
	}
	v, ok := value.(string)
	if !ok {
		b, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("MergeStatus: expected string, got %T", value)
		}
		v = string(b)
	}
	*s = MergeStatus(v)
	return nil
}

// Value implements driver.Valuer.
func (s MergeStatus) Value() (driver.Value, error) { return string(s), nil }

// MarshalJSON implements json.Marshaler.
func (s MergeStatus) MarshalJSON() ([]byte, error) { return json.Marshal(string(s)) }

// UnmarshalJSON implements json.Unmarshaler.
func (s *MergeStatus) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = MergeStatus(v)
	return nil
}

// ScratchType represents the type discriminator for scratch payloads.
// Matches Rust strumDiscriminants rename_all = "SCREAMING_SNAKE_CASE".
type ScratchType string

const (
	ScratchTypeDraftTask          ScratchType = "DRAFT_TASK"
	ScratchTypeDraftFollowUp      ScratchType = "DRAFT_FOLLOW_UP"
	ScratchTypeDraftWorkspace     ScratchType = "DRAFT_WORKSPACE"
	ScratchTypeDraftIssue         ScratchType = "DRAFT_ISSUE"
	ScratchTypePreviewSettings    ScratchType = "PREVIEW_SETTINGS"
	ScratchTypeWorkspaceNotes     ScratchType = "WORKSPACE_NOTES"
	ScratchTypeUIPreferences      ScratchType = "UI_PREFERENCES"
	ScratchTypeProjectRepoDefaults ScratchType = "PROJECT_REPO_DEFAULTS"
)

func (t ScratchType) String() string { return string(t) }

// MergeType represents the type discriminator for Merge variants.
// Matches Rust serde rename_all = "snake_case".
type MergeType string

const (
	MergeTypeDirect MergeType = "direct"
	MergeTypePr     MergeType = "pr"
)

func (t MergeType) String() string { return string(t) }
