package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSON Pointer escaping per RFC 6901.
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// WorkspacePatch creates a patch for a workspace change.
func WorkspacePatch(op, id string, value any) PatchOperation {
	path := fmt.Sprintf("/workspaces/%s", escapeJSONPointer(id))
	return PatchOperation{Op: op, Path: path, Value: value}
}

// WorkspaceSnapshotPatch creates a replace patch for the full workspace snapshot.
func WorkspaceSnapshotPatch(value any) PatchOperation {
	return PatchOperation{Op: "replace", Path: "/workspaces", Value: value}
}

// ExecutionProcessPatch creates a patch for an execution process change.
func ExecutionProcessPatch(op, id string, value any) PatchOperation {
	path := fmt.Sprintf("/execution_processes/%s", escapeJSONPointer(id))
	return PatchOperation{Op: op, Path: path, Value: value}
}

// ExecutionProcessSnapshotPatch creates a replace patch for the full snapshot.
func ExecutionProcessSnapshotPatch(value any) PatchOperation {
	return PatchOperation{Op: "replace", Path: "/execution_processes", Value: value}
}

// ScratchPatch creates a patch targeting the shared /scratch path.
func ScratchPatch(op string, value any) PatchOperation {
	return PatchOperation{Op: op, Path: "/scratch", Value: value}
}

// ScratchRemovePatch creates a scratch remove with a deleted marker.
func ScratchRemovePatch(id string, scratchType string) PatchOperation {
	return PatchOperation{
		Op:    "replace",
		Path:  "/scratch",
		Value: map[string]any{"id": id, "payload": map[string]string{"type": scratchType}, "deleted": true},
	}
}

// ApprovalsSnapshotPatch creates a replace patch for the full pending approvals map.
func ApprovalsSnapshotPatch(value any) PatchOperation {
	return PatchOperation{Op: "replace", Path: "/pending", Value: value}
}

// ApprovalCreatedPatch creates an add patch for a new approval.
func ApprovalCreatedPatch(id string, value any) PatchOperation {
	return PatchOperation{Op: "add", Path: fmt.Sprintf("/pending/%s", escapeJSONPointer(id)), Value: value}
}

// ApprovalResolvedPatch creates a remove patch for a resolved approval.
func ApprovalResolvedPatch(id string) PatchOperation {
	return PatchOperation{Op: "remove", Path: fmt.Sprintf("/pending/%s", escapeJSONPointer(id))}
}

// PatchPathMatches checks if a patch path starts with the given prefix.
func PatchPathMatches(patch PatchOperation, prefix string) bool {
	return strings.HasPrefix(patch.Path, prefix)
}

// PatchPathHasPrefix checks if a patch path matches exactly or starts with prefix/.
func PatchPathHasPrefix(patch PatchOperation, prefix string) bool {
	return patch.Path == prefix || strings.HasPrefix(patch.Path, prefix+"/")
}

// MarshalPatchValue marshals a patch value to JSON bytes.
func MarshalPatchValue(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
