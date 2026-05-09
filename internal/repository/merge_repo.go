package repository

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// MergeRepo provides data access for merges (both direct and PR-based).
type MergeRepo struct {
	db *sql.DB
}

// NewMergeRepo creates a new MergeRepo.
func NewMergeRepo(db *sql.DB) *MergeRepo {
	return &MergeRepo{db: db}
}

// FindByWorkspaceID returns all merges (direct and PR) for a workspace, sorted by created_at DESC.
func (r *MergeRepo) FindByWorkspaceID(workspaceID domain.UUID) ([]domain.Merge, error) {
	// Query direct merges
	directRows, err := r.db.Query(`
		SELECT id, workspace_id, repo_id, merge_commit, target_branch_name, created_at
		FROM merges
		WHERE workspace_id = ?
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find direct merges by workspace id: %w", err)
	}
	defer directRows.Close()

	var merges []domain.Merge
	for directRows.Next() {
		var id, wsID, repoID domain.UUID
		var mergeCommit, targetBranch string
		var createdAt time.Time
		if err := directRows.Scan(&id, &wsID, &repoID, &mergeCommit, &targetBranch, timeScanner{&createdAt}); err != nil {
			return nil, fmt.Errorf("scan direct merge: %w", err)
		}
		merges = append(merges, domain.Merge{
			Type:             domain.MergeTypeDirect,
			ID:               id,
			WorkspaceID:      wsID,
			RepoID:           repoID,
			TargetBranchName: targetBranch,
			CreatedAt:        createdAt,
			MergeCommit:      &mergeCommit,
		})
	}
	if err := directRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate direct merges: %w", err)
	}

	// Query PR merges
	prRows, err := r.db.Query(`
		SELECT id, workspace_id, repo_id, pr_url, pr_number, pr_status,
		       target_branch_name, merged_at, merge_commit_sha, created_at
		FROM pull_requests
		WHERE workspace_id = ?
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find pr merges by workspace id: %w", err)
	}
	defer prRows.Close()

	for prRows.Next() {
		var pr domain.PullRequest
		var idStr string
		if err := prRows.Scan(
			&idStr, &pr.WorkspaceID, &pr.RepoID, &pr.PRURL, &pr.PRNumber, &pr.PRStatus,
			&pr.TargetBranchName, nullTimeScanner{&pr.MergedAt}, &pr.MergeCommitSHA, &pr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pull request: %w", err)
		}
		pr.ID = idStr
		prMerge := pr.ToPrMerge()
		merges = append(merges, domain.NewPrMerge(prMerge))
	}
	if err := prRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pr merges: %w", err)
	}

	// Sort all merges by created_at DESC
	sort.Slice(merges, func(i, j int) bool {
		return merges[i].CreatedAt.After(merges[j].CreatedAt)
	})

	return merges, nil
}

// CreateDirectMerge inserts a direct merge into the merges table.
func (r *MergeRepo) CreateDirectMerge(m *domain.DirectMerge) error {
	_, err := r.db.Exec(`
		INSERT INTO merges (id, workspace_id, repo_id, merge_type, merge_commit, target_branch_name, created_at)
		VALUES (?, ?, ?, 'direct', ?, ?, ?)
	`, m.ID[:], m.WorkspaceID[:], m.RepoID[:], m.MergeCommit, m.TargetBranchName, m.CreatedAt)
	if err != nil {
		return fmt.Errorf("create direct merge: %w", err)
	}
	return nil
}

// CreatePrMerge inserts a PR-based merge into the pull_requests table.
// Uses ON CONFLICT(pr_url) DO UPDATE to upsert.
func (r *MergeRepo) CreatePrMerge(pr *domain.PullRequest) error {
	_, err := r.db.Exec(`
		INSERT INTO pull_requests (id, workspace_id, repo_id, pr_url, pr_number, pr_status,
		                           target_branch_name, merged_at, merge_commit_sha, created_at, updated_at, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pr_url) DO UPDATE SET
			pr_status = excluded.pr_status,
			merged_at = excluded.merged_at,
			merge_commit_sha = excluded.merge_commit_sha,
			updated_at = excluded.updated_at,
			synced_at = excluded.synced_at
	`, pr.ID, nullUUID(pr.WorkspaceID), nullUUID(pr.RepoID),
		pr.PRURL, pr.PRNumber, pr.PRStatus,
		pr.TargetBranchName, nullTime(pr.MergedAt), nullString(pr.MergeCommitSHA),
		pr.CreatedAt, pr.UpdatedAt, nullTime(pr.SyncedAt))
	if err != nil {
		return fmt.Errorf("create pr merge: %w", err)
	}
	return nil
}

// UpdatePrStatus updates the status, merged_at, and merge_commit_sha of a pull request by pr_url.
func (r *MergeRepo) UpdatePrStatus(prURL string, status domain.MergeStatus, mergedAt *time.Time, mergeCommitSHA *string) error {
	_, err := r.db.Exec(`
		UPDATE pull_requests
		SET pr_status = ?, merged_at = ?, merge_commit_sha = ?, updated_at = datetime('now', 'subsec')
		WHERE pr_url = ?
	`, status, nullTime(mergedAt), nullString(mergeCommitSHA), prURL)
	if err != nil {
		return fmt.Errorf("update pr status: %w", err)
	}
	return nil
}
