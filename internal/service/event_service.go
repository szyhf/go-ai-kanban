package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// HookTable identifies which table triggered a change.
type HookTable string

const (
	HookTableWorkspaces         HookTable = "workspaces"
	HookTableExecutionProcesses HookTable = "execution_processes"
	HookTableScratch            HookTable = "scratch"
)

// HookOp identifies the database operation type.
type HookOp string

const (
	HookOpInsert HookOp = "insert"
	HookOpUpdate HookOp = "update"
	HookOpDelete HookOp = "delete"
)

// EventService watches for database changes and broadcasts JSON Patch events.
type EventService struct {
	msgStore *MsgStore
	db       *sql.DB
}

// NewEventService creates a new EventService.
func NewEventService(msgStore *MsgStore, db *sql.DB) *EventService {
	return &EventService{msgStore: msgStore, db: db}
}

// NotifyChange is called by the repository layer after a write operation.
// It loads the affected record and pushes the appropriate patch.
func (s *EventService) NotifyChange(table HookTable, op HookOp, id domain.UUID) {
	switch table {
	case HookTableWorkspaces:
		s.notifyWorkspaceChange(op, id)
	case HookTableExecutionProcesses:
		s.notifyExecutionProcessChange(op, id)
	case HookTableScratch:
		s.notifyScratchChange(op, id)
	default:
		slog.Debug("unknown hook table", "table", table)
	}
}

// NotifyDelete is called for delete operations where we can't load the record.
func (s *EventService) NotifyDelete(table HookTable, id domain.UUID) {
	idStr := id.String()
	var patch PatchOperation
	switch table {
	case HookTableWorkspaces:
		patch = WorkspacePatch("remove", idStr, nil)
	case HookTableExecutionProcesses:
		patch = ExecutionProcessPatch("remove", idStr, nil)
	case HookTableScratch:
		patch = ScratchRemovePatch(idStr, "")
	default:
		return
	}
	s.msgStore.PushPatch(patch)
}

// PushSnapshot pushes the initial state for a new subscriber.
func (s *EventService) PushSnapshot(patch PatchOperation) {
	s.msgStore.PushPatch(patch)
}

// StreamWorkspaces pushes an initial workspace snapshot then returns the subscription.
// The caller filters the broadcast channel for workspace patches.
func (s *EventService) StreamWorkspaces() (<-chan LogMsg, func()) {
	return s.msgStore.Subscribe()
}

// StreamExecutionProcesses pushes initial state and returns a subscription.
func (s *EventService) StreamExecutionProcesses() (<-chan LogMsg, func()) {
	return s.msgStore.Subscribe()
}

// StreamScratch pushes initial state and returns a subscription.
func (s *EventService) StreamScratch() (<-chan LogMsg, func()) {
	return s.msgStore.Subscribe()
}

// FilteredSubscribe returns a channel that only receives patches matching the given path prefix.
func (s *EventService) FilteredSubscribe(pathPrefix string) (<-chan LogMsg, func()) {
	ch, unsub := s.msgStore.Subscribe()
	filtered := make(chan LogMsg, subscriberBuffer)

	go func() {
		defer close(filtered)
		for msg := range ch {
			if msg.Kind == LogMsgPatch && msg.Patch != nil {
				if PatchPathHasPrefix(*msg.Patch, pathPrefix) {
					select {
					case filtered <- msg:
					default:
					}
				}
			} else if msg.Kind == LogMsgReady || msg.Kind == LogMsgFinished {
				select {
				case filtered <- msg:
				default:
				}
			}
		}
	}()

	return filtered, unsub
}

// notifyWorkspaceChange handles workspace insert/update events.
func (s *EventService) notifyWorkspaceChange(op HookOp, id domain.UUID) {
	// Load the workspace from DB.
	var wsJSON json.RawMessage
	err := s.db.QueryRow(`
		SELECT json_object(
			'id', hex(id),
			'name', name,
			'path', path,
			'repo_id', COALESCE(hex(repo_id), ''),
			'branch', COALESCE(branch, ''),
			'status', COALESCE(status, ''),
			'archived', archived,
			'created_at', created_at,
			'updated_at', updated_at
		)
		FROM workspaces WHERE id = ?
	`, id[:]).Scan(&wsJSON)
	if err != nil {
		slog.Warn("event: load workspace", "id", id, "error", err)
		return
	}

	var value interface{}
	if err := json.Unmarshal(wsJSON, &value); err != nil {
		slog.Warn("event: unmarshal workspace", "error", err)
		return
	}

	patchOp := "replace"
	if op == HookOpInsert {
		patchOp = "add"
	}
	s.msgStore.PushPatch(WorkspacePatch(patchOp, id.String(), value))
}

// notifyExecutionProcessChange handles execution process insert/update events.
func (s *EventService) notifyExecutionProcessChange(op HookOp, id domain.UUID) {
	var epJSON json.RawMessage
	err := s.db.QueryRow(`
		SELECT json_object(
			'id', hex(id),
			'session_id', hex(session_id),
			'process_id', process_id,
			'status', COALESCE(status, ''),
			'created_at', created_at,
			'updated_at', updated_at
		)
		FROM execution_processes WHERE id = ?
	`, id[:]).Scan(&epJSON)
	if err != nil {
		slog.Warn("event: load execution process", "id", id, "error", err)
		return
	}

	var value interface{}
	if err := json.Unmarshal(epJSON, &value); err != nil {
		slog.Warn("event: unmarshal execution process", "error", err)
		return
	}

	patchOp := "replace"
	if op == HookOpInsert {
		patchOp = "add"
	}
	s.msgStore.PushPatch(ExecutionProcessPatch(patchOp, id.String(), value))
}

// notifyScratchChange handles scratch insert/update events.
func (s *EventService) notifyScratchChange(op HookOp, id domain.UUID) {
	var scratchJSON json.RawMessage
	err := s.db.QueryRow(`
		SELECT json_object(
			'id', hex(id),
			'scratch_id', scratch_id,
			'type', COALESCE(type, ''),
			'content', COALESCE(content, ''),
			'updated_at', updated_at
		)
		FROM scratch WHERE id = ?
	`, id[:]).Scan(&scratchJSON)
	if err != nil {
		slog.Warn("event: load scratch", "id", id, "error", err)
		return
	}

	var value interface{}
	if err := json.Unmarshal(scratchJSON, &value); err != nil {
		slog.Warn("event: unmarshal scratch", "error", err)
		return
	}

	s.msgStore.PushPatch(ScratchPatch("replace", value))
}

// String implements fmt.Stringer.
func (s *EventService) String() string {
	return fmt.Sprintf("EventService{msgStore: %s}", s.msgStore)
}

// PatchPath extracts the JSON Pointer path from a LogMsg.
func PatchPath(msg LogMsg) string {
	if msg.Patch != nil {
		return msg.Patch.Path
	}
	return ""
}

// IsWorkspacePatch checks if a LogMsg is a workspace-related patch.
func IsWorkspacePatch(msg LogMsg) bool {
	return msg.Kind == LogMsgPatch && msg.Patch != nil &&
		strings.HasPrefix(msg.Patch.Path, "/workspaces")
}

// IsExecutionProcessPatch checks if a LogMsg is an execution process patch.
func IsExecutionProcessPatch(msg LogMsg) bool {
	return msg.Kind == LogMsgPatch && msg.Patch != nil &&
		strings.HasPrefix(msg.Patch.Path, "/execution_processes")
}

// IsScratchPatch checks if a LogMsg is a scratch patch.
func IsScratchPatch(msg LogMsg) bool {
	return msg.Kind == LogMsgPatch && msg.Patch != nil &&
		msg.Patch.Path == "/scratch"
}
