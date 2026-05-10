package git

import "strings"

// DiffChangeKind represents the kind of change in a diff.
type DiffChangeKind string

const (
	DiffChangeAdded    DiffChangeKind = "added"
	DiffChangeModified DiffChangeKind = "modified"
	DiffChangeDeleted  DiffChangeKind = "deleted"
	DiffChangeRenamed  DiffChangeKind = "renamed"
)

// Diff represents a single file diff.
type Diff struct {
	Change         DiffChangeKind `json:"change"`
	OldPath        string         `json:"oldPath"`
	NewPath        string         `json:"newPath"`
	OldContent     string         `json:"oldContent"`
	NewContent     string         `json:"newContent"`
	ContentOmitted bool           `json:"contentOmitted"`
	Additions      int            `json:"additions"`
	Deletions      int            `json:"deletions"`
	RepoID         string         `json:"repoId"`
}

// MaxInlineDiffBytes is the maximum size of file content to include inline.
// Files larger than this will have ContentOmitted set to true.
const MaxInlineDiffBytes = 2 * 1024 * 1024 // 2 MB

// DiffPath returns the display path for a diff.
func DiffPath(d *Diff) string {
	if d.NewPath != "" {
		return d.NewPath
	}
	if d.OldPath != "" {
		return d.OldPath
	}
	return ""
}

// ComputeLineChangeCounts computes the number of added and deleted lines
// between old and new content.
func ComputeLineChangeCounts(old, newContent string) (additions, deletions int) {
	if old == newContent {
		return 0, 0
	}

	oldLines := splitLines(old)
	newLines := splitLines(newContent)

	// Simple line diff using LCS-free approach: count unique old and new lines.
	oldSet := make(map[string]int)
	for _, line := range oldLines {
		oldSet[line]++
	}

	newSet := make(map[string]int)
	for _, line := range newLines {
		newSet[line]++
	}

	// Deletions: lines in old but not matched in new.
	for line, oldCount := range oldSet {
		newCount := newSet[line]
		if oldCount > newCount {
			deletions += oldCount - newCount
		}
	}

	// Additions: lines in new but not matched in old.
	for line, newCount := range newSet {
		oldCount := oldSet[line]
		if newCount > oldCount {
			additions += newCount - oldCount
		}
	}

	return additions, deletions
}

// splitLines splits content into lines (including empty lines).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	result := strings.Split(s, "\n")
	// Remove trailing empty string from split if content ends with newline.
	if len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return result
}
