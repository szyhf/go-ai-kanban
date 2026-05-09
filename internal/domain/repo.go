package domain

import (
	"encoding/json"
	"time"
)

// Repo represents a git repository tracked by the system.
// Matches Rust crates/db/src/models/repo.rs.
type Repo struct {
	ID                   UUID      `json:"id"`
	Path                 string    `json:"path"`
	Name                 string    `json:"name"`
	DisplayName          string    `json:"display_name"`
	SetupScript          *string   `json:"setup_script"`
	CleanupScript        *string   `json:"cleanup_script"`
	ArchiveScript        *string   `json:"archive_script"`
	CopyFiles            *string   `json:"copy_files"`
	ParallelSetupScript  bool      `json:"parallel_setup_script"`
	DevServerScript      *string   `json:"dev_server_script"`
	DefaultTargetBranch  *string   `json:"default_target_branch"`
	DefaultWorkingDir    *string   `json:"default_working_dir"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UpdateRepo is the request DTO for updating a repo.
// Uses double-option pattern: nil = don't update, pointer to nil = set to NULL.
type UpdateRepo struct {
	DisplayName         **string `json:"display_name"`
	SetupScript         **string `json:"setup_script"`
	CleanupScript       **string `json:"cleanup_script"`
	ArchiveScript       **string `json:"archive_script"`
	CopyFiles           **string `json:"copy_files"`
	ParallelSetupScript **bool   `json:"parallel_setup_script"`
	DevServerScript     **string `json:"dev_server_script"`
	DefaultTargetBranch **string `json:"default_target_branch"`
	DefaultWorkingDir   **string `json:"default_working_dir"`
}

// SearchMatchType represents what kind of search match was found.
type SearchMatchType string

const (
	SearchMatchFileName       SearchMatchType = "FileName"
	SearchMatchDirectoryName  SearchMatchType = "DirectoryName"
	SearchMatchFullPath       SearchMatchType = "FullPath"
)

// SearchResult represents a file/directory search result.
type SearchResult struct {
	Path      string          `json:"path"`
	IsFile    bool            `json:"is_file"`
	MatchType SearchMatchType `json:"match_type"`
	Score     int64           `json:"score"`
}

// RepoWithTargetBranch embeds Repo and adds a target branch.
// Matches Rust's RepoWithTargetBranch with #[serde(flatten)].
type RepoWithTargetBranch struct {
	Repo
	TargetBranch string `json:"target_branch"`
}

// RepoWithCopyFiles is an internal type for repo copy operations.
type RepoWithCopyFiles struct {
	ID        UUID
	Path      string
	Name      string
	CopyFiles *string
}

// ExecutorActionField wraps an ExecutorAction stored as JSON in the database.
// Matches Rust's untagged ExecutorActionField enum.
type ExecutorActionField struct {
	Raw json.RawMessage
}
