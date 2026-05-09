package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// PullRequestInfo contains PR metadata for a PR-based merge.
// Matches Rust crates/db/src/models/merge.rs PullRequestInfo.
type PullRequestInfo struct {
	Number          int64      `json:"number"`
	URL             string     `json:"url"`
	Status          MergeStatus `json:"status"`
	MergedAt        *time.Time `json:"merged_at"`
	MergeCommitSHA  *string    `json:"merge_commit_sha"`
}

// DirectMerge represents a direct (non-PR) merge.
// Matches Rust DirectMerge variant.
type DirectMerge struct {
	ID               UUID      `json:"id"`
	WorkspaceID      UUID      `json:"workspace_id"`
	RepoID           UUID      `json:"repo_id"`
	MergeCommit      string    `json:"merge_commit"`
	TargetBranchName string    `json:"target_branch_name"`
	CreatedAt        time.Time `json:"created_at"`
}

// PrMerge represents a PR-based merge.
// Matches Rust PrMerge variant.
type PrMerge struct {
	ID               UUID            `json:"id"`
	WorkspaceID      UUID            `json:"workspace_id"`
	RepoID           UUID            `json:"repo_id"`
	CreatedAt        time.Time       `json:"created_at"`
	TargetBranchName string          `json:"target_branch_name"`
	PrInfo           PullRequestInfo `json:"pr_info"`
}

// Merge represents a tagged union of DirectMerge and PrMerge.
// Matches Rust #[serde(tag = "type", rename_all = "snake_case")].
// JSON: {"type": "direct", ...} or {"type": "pr", ...}
type Merge struct {
	Type              MergeType `json:"type"`
	ID                UUID      `json:"id"`
	WorkspaceID       UUID      `json:"workspace_id"`
	RepoID            UUID      `json:"repo_id"`
	TargetBranchName  string    `json:"target_branch_name"`
	CreatedAt         time.Time `json:"created_at"`
	// Direct merge only
	MergeCommit *string `json:"merge_commit,omitempty"`
	// PR merge only
	PrInfo *PullRequestInfo `json:"pr_info,omitempty"`
}

// NewDirectMerge creates a Merge from a DirectMerge.
func NewDirectMerge(d DirectMerge) Merge {
	return Merge{
		Type:             MergeTypeDirect,
		ID:               d.ID,
		WorkspaceID:      d.WorkspaceID,
		RepoID:           d.RepoID,
		TargetBranchName: d.TargetBranchName,
		CreatedAt:        d.CreatedAt,
		MergeCommit:      &d.MergeCommit,
	}
}

// NewPrMerge creates a Merge from a PrMerge.
func NewPrMerge(p PrMerge) Merge {
	return Merge{
		Type:             MergeTypePr,
		ID:               p.ID,
		WorkspaceID:      p.WorkspaceID,
		RepoID:           p.RepoID,
		TargetBranchName: p.TargetBranchName,
		CreatedAt:        p.CreatedAt,
		PrInfo:           &p.PrInfo,
	}
}

// AsDirect returns the DirectMerge if type is direct, nil otherwise.
func (m Merge) AsDirect() *DirectMerge {
	if m.Type != MergeTypeDirect || m.MergeCommit == nil {
		return nil
	}
	return &DirectMerge{
		ID:               m.ID,
		WorkspaceID:      m.WorkspaceID,
		RepoID:           m.RepoID,
		MergeCommit:      *m.MergeCommit,
		TargetBranchName: m.TargetBranchName,
		CreatedAt:        m.CreatedAt,
	}
}

// AsPr returns the PrMerge if type is pr, nil otherwise.
func (m Merge) AsPr() *PrMerge {
	if m.Type != MergeTypePr || m.PrInfo == nil {
		return nil
	}
	return &PrMerge{
		ID:               m.ID,
		WorkspaceID:      m.WorkspaceID,
		RepoID:           m.RepoID,
		CreatedAt:        m.CreatedAt,
		TargetBranchName: m.TargetBranchName,
		PrInfo:           *m.PrInfo,
	}
}

// PullRequest represents a tracked pull request (DB-only, not directly serialized).
// Matches Rust crates/db/src/models/pull_request.rs.
// Note: ID is string (hex-encoded), not UUID.
type PullRequest struct {
	ID               string      `json:"id"`
	WorkspaceID      *UUID       `json:"workspace_id"`
	RepoID           *UUID       `json:"repo_id"`
	PRURL            string      `json:"pr_url"`
	PRNumber         int64       `json:"pr_number"`
	PRStatus         MergeStatus `json:"pr_status"`
	TargetBranchName string      `json:"target_branch_name"`
	MergedAt         *time.Time  `json:"merged_at"`
	MergeCommitSHA   *string     `json:"merge_commit_sha"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	SyncedAt         *time.Time  `json:"synced_at"`
}

// ToPrMerge converts a PullRequest to a PrMerge for API response.
func (pr *PullRequest) ToPrMerge() PrMerge {
	info := PullRequestInfo{
		Number: pr.PRNumber,
		URL:    pr.PRURL,
		Status: pr.PRStatus,
	}
	if pr.MergedAt != nil {
		info.MergedAt = pr.MergedAt
	}
	if pr.MergeCommitSHA != nil {
		info.MergeCommitSHA = pr.MergeCommitSHA
	}
	return PrMerge{
		ID:               MustParseUUID(pr.ID),
		WorkspaceID:      *pr.WorkspaceID,
		RepoID:           *pr.RepoID,
		CreatedAt:        pr.CreatedAt,
		TargetBranchName: pr.TargetBranchName,
		PrInfo:           info,
	}
}

// UnmarshalJSON implements custom unmarshaling for Merge tagged union.
func (m *Merge) UnmarshalJSON(data []byte) error {
	// First pass: extract the type field
	var raw struct {
		Type MergeType `json:"type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("merge: missing type field: %w", err)
	}
	m.Type = raw.Type

	switch raw.Type {
	case MergeTypeDirect:
		var d DirectMerge
		if err := json.Unmarshal(data, &d); err != nil {
			return fmt.Errorf("merge direct: %w", err)
		}
		*m = NewDirectMerge(d)
	case MergeTypePr:
		var p PrMerge
		if err := json.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("merge pr: %w", err)
		}
		*m = NewPrMerge(p)
	default:
		return fmt.Errorf("merge: unknown type %q", raw.Type)
	}
	return nil
}
