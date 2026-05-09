package repository

import (
	"testing"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

func TestMergeRepo_CreateDirectMerge(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	mergeRepo := NewMergeRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)
	repo, err := gitRepo.FindOrCreate("/path/merge-repo", "merge-repo", "Merge Repo")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	dm := &domain.DirectMerge{
		ID:               domain.NewUUID(),
		WorkspaceID:      wsID,
		RepoID:           repo.ID,
		MergeCommit:      "abc123def456",
		TargetBranchName: "main",
		CreatedAt:        time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := mergeRepo.CreateDirectMerge(dm); err != nil {
		t.Fatalf("CreateDirectMerge: %v", err)
	}

	merges, err := mergeRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(merges) != 1 {
		t.Fatalf("expected 1 merge, got %d", len(merges))
	}

	m := merges[0]
	if m.Type != domain.MergeTypeDirect {
		t.Errorf("Type: got %q, want %q", m.Type, domain.MergeTypeDirect)
	}
	if m.ID != dm.ID {
		t.Errorf("ID: got %s, want %s", m.ID, dm.ID)
	}
	if m.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %s, want %s", m.WorkspaceID, wsID)
	}
	if m.RepoID != repo.ID {
		t.Errorf("RepoID: got %s, want %s", m.RepoID, repo.ID)
	}
	if m.MergeCommit == nil || *m.MergeCommit != "abc123def456" {
		t.Errorf("MergeCommit: got %v, want %q", m.MergeCommit, "abc123def456")
	}
	if m.TargetBranchName != "main" {
		t.Errorf("TargetBranchName: got %q, want %q", m.TargetBranchName, "main")
	}
}

func TestMergeRepo_CreatePrMerge(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	mergeRepo := NewMergeRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)
	repo, err := gitRepo.FindOrCreate("/path/pr-repo", "pr-repo", "PR Repo")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	pr := &domain.PullRequest{
		ID:               domain.NewUUID().String(),
		WorkspaceID:      &wsID,
		RepoID:           &repo.ID,
		PRURL:            "https://github.com/test/repo/pull/1",
		PRNumber:         1,
		PRStatus:         domain.MergeStatusOpen,
		TargetBranchName: "main",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := mergeRepo.CreatePrMerge(pr); err != nil {
		t.Fatalf("CreatePrMerge: %v", err)
	}

	merges, err := mergeRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(merges) != 1 {
		t.Fatalf("expected 1 merge, got %d", len(merges))
	}

	m := merges[0]
	if m.Type != domain.MergeTypePr {
		t.Errorf("Type: got %q, want %q", m.Type, domain.MergeTypePr)
	}
	if m.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %s, want %s", m.WorkspaceID, wsID)
	}
	if m.RepoID != repo.ID {
		t.Errorf("RepoID: got %s, want %s", m.RepoID, repo.ID)
	}
	if m.PrInfo == nil {
		t.Fatal("PrInfo: got nil, expected non-nil")
	}
	if m.PrInfo.URL != "https://github.com/test/repo/pull/1" {
		t.Errorf("PrInfo.URL: got %q, want %q", m.PrInfo.URL, "https://github.com/test/repo/pull/1")
	}
	if m.PrInfo.Number != 1 {
		t.Errorf("PrInfo.Number: got %d, want %d", m.PrInfo.Number, 1)
	}
	if m.PrInfo.Status != domain.MergeStatusOpen {
		t.Errorf("PrInfo.Status: got %q, want %q", m.PrInfo.Status, domain.MergeStatusOpen)
	}
	if m.TargetBranchName != "main" {
		t.Errorf("TargetBranchName: got %q, want %q", m.TargetBranchName, "main")
	}
}

func TestMergeRepo_FindByWorkspaceID_Mixed(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	mergeRepo := NewMergeRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)
	repo, err := gitRepo.FindOrCreate("/path/mixed-repo", "mixed-repo", "Mixed Repo")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	baseTime := time.Now().UTC().Truncate(time.Microsecond)

	// Create a direct merge.
	dm := &domain.DirectMerge{
		ID:               domain.NewUUID(),
		WorkspaceID:      wsID,
		RepoID:           repo.ID,
		MergeCommit:      "direct123",
		TargetBranchName: "main",
		CreatedAt:        baseTime,
	}
	if err := mergeRepo.CreateDirectMerge(dm); err != nil {
		t.Fatalf("CreateDirectMerge: %v", err)
	}

	// Create a PR merge (slightly later).
	pr := &domain.PullRequest{
		ID:               domain.NewUUID().String(),
		WorkspaceID:      &wsID,
		RepoID:           &repo.ID,
		PRURL:            "https://github.com/test/repo/pull/2",
		PRNumber:         2,
		PRStatus:         domain.MergeStatusOpen,
		TargetBranchName: "develop",
		CreatedAt:        baseTime.Add(time.Second),
		UpdatedAt:        baseTime.Add(time.Second),
	}
	if err := mergeRepo.CreatePrMerge(pr); err != nil {
		t.Fatalf("CreatePrMerge: %v", err)
	}

	merges, err := mergeRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(merges) != 2 {
		t.Fatalf("expected 2 merges, got %d", len(merges))
	}

	// Sorted by created_at DESC, so the PR merge (later) comes first.
	if merges[0].Type != domain.MergeTypePr {
		t.Errorf("merges[0].Type: got %q, want %q", merges[0].Type, domain.MergeTypePr)
	}
	if merges[1].Type != domain.MergeTypeDirect {
		t.Errorf("merges[1].Type: got %q, want %q", merges[1].Type, domain.MergeTypeDirect)
	}
}

func TestMergeRepo_UpdatePrStatus(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	mergeRepo := NewMergeRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)
	repo, err := gitRepo.FindOrCreate("/path/status-repo", "status-repo", "Status Repo")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	pr := &domain.PullRequest{
		ID:               domain.NewUUID().String(),
		WorkspaceID:      &wsID,
		RepoID:           &repo.ID,
		PRURL:            "https://github.com/test/repo/pull/3",
		PRNumber:         3,
		PRStatus:         domain.MergeStatusOpen,
		TargetBranchName: "main",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := mergeRepo.CreatePrMerge(pr); err != nil {
		t.Fatalf("CreatePrMerge: %v", err)
	}

	// Update to merged.
	mergedAt := now.Add(5 * time.Minute)
	mergeCommitSHA := "sha789"
	if err := mergeRepo.UpdatePrStatus(pr.PRURL, domain.MergeStatusMerged, &mergedAt, &mergeCommitSHA); err != nil {
		t.Fatalf("UpdatePrStatus: %v", err)
	}

	merges, err := mergeRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(merges) != 1 {
		t.Fatalf("expected 1 merge, got %d", len(merges))
	}

	m := merges[0]
	if m.PrInfo == nil {
		t.Fatal("PrInfo: got nil, expected non-nil")
	}
	if m.PrInfo.Status != domain.MergeStatusMerged {
		t.Errorf("PrInfo.Status: got %q, want %q", m.PrInfo.Status, domain.MergeStatusMerged)
	}
	if m.PrInfo.MergedAt == nil {
		t.Error("PrInfo.MergedAt: got nil, expected non-nil")
	}
	if m.PrInfo.MergeCommitSHA == nil || *m.PrInfo.MergeCommitSHA != "sha789" {
		t.Errorf("PrInfo.MergeCommitSHA: got %v, want %q", m.PrInfo.MergeCommitSHA, "sha789")
	}
}

func TestMergeRepo_CreatePrMerge_Upsert(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	mergeRepo := NewMergeRepo(db.DB)
	gitRepo := NewGitRepoRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)
	repo, err := gitRepo.FindOrCreate("/path/upsert-repo", "upsert-repo", "Upsert Repo")
	if err != nil {
		t.Fatalf("FindOrCreate repo: %v", err)
	}

	prURL := "https://github.com/test/repo/pull/4"
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Create initial PR with status=open.
	pr1 := &domain.PullRequest{
		ID:               domain.NewUUID().String(),
		WorkspaceID:      &wsID,
		RepoID:           &repo.ID,
		PRURL:            prURL,
		PRNumber:         4,
		PRStatus:         domain.MergeStatusOpen,
		TargetBranchName: "main",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := mergeRepo.CreatePrMerge(pr1); err != nil {
		t.Fatalf("CreatePrMerge (first): %v", err)
	}

	// Upsert with same pr_url but different status.
	mergedAt := now.Add(10 * time.Minute)
	mergeSHA := "upsertsha"
	pr2 := &domain.PullRequest{
		ID:               domain.NewUUID().String(),
		WorkspaceID:      &wsID,
		RepoID:           &repo.ID,
		PRURL:            prURL,
		PRNumber:         4,
		PRStatus:         domain.MergeStatusMerged,
		TargetBranchName: "main",
		MergedAt:         &mergedAt,
		MergeCommitSHA:   &mergeSHA,
		CreatedAt:        now,
		UpdatedAt:        now.Add(10 * time.Minute),
	}
	if err := mergeRepo.CreatePrMerge(pr2); err != nil {
		t.Fatalf("CreatePrMerge (upsert): %v", err)
	}

	// Should still be only 1 merge (upserted, not duplicated).
	merges, err := mergeRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(merges) != 1 {
		t.Fatalf("expected 1 merge after upsert, got %d", len(merges))
	}

	m := merges[0]
	if m.PrInfo == nil {
		t.Fatal("PrInfo: got nil, expected non-nil")
	}
	if m.PrInfo.Status != domain.MergeStatusMerged {
		t.Errorf("PrInfo.Status: got %q, want %q", m.PrInfo.Status, domain.MergeStatusMerged)
	}
	if m.PrInfo.MergeCommitSHA == nil || *m.PrInfo.MergeCommitSHA != "upsertsha" {
		t.Errorf("PrInfo.MergeCommitSHA: got %v, want %q", m.PrInfo.MergeCommitSHA, "upsertsha")
	}
}
