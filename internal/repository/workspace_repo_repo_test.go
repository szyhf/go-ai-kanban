package repository

import (
	"database/sql"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// createTestWorkspace inserts a minimal workspace row and returns its ID.
// This is needed because workspace_repos has a FK to workspaces.
func createTestWorkspace(t *testing.T, db *sql.DB) domain.UUID {
	t.Helper()
	id := domain.NewUUID()
	_, err := db.Exec(`
		INSERT INTO workspaces (id, branch, archived, pinned, worktree_deleted)
		VALUES (?, ?, 0, 0, 0)
	`, id[:], "test-branch")
	if err != nil {
		t.Fatalf("insert test workspace: %v", err)
	}
	return id
}

func TestWorkspaceRepoRepo_CreateMany_FindByWorkspaceID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	gitRepo := NewGitRepoRepo(db.DB)
	wsRepo := NewWorkspaceRepoRepo(db.DB)

	// Create repos.
	r1, err := gitRepo.FindOrCreate("/path/r1", "repo1", "Repo 1")
	if err != nil {
		t.Fatalf("FindOrCreate r1: %v", err)
	}
	r2, err := gitRepo.FindOrCreate("/path/r2", "repo2", "Repo 2")
	if err != nil {
		t.Fatalf("FindOrCreate r2: %v", err)
	}

	// Create a workspace.
	wsID := createTestWorkspace(t, db.DB)

	// Create junction entries.
	err = wsRepo.CreateMany(wsID, []domain.CreateWorkspaceRepo{
		{RepoID: r1.ID, TargetBranch: "feature-a"},
		{RepoID: r2.ID, TargetBranch: "feature-b"},
	})
	if err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	// FindByWorkspaceID should return both.
	results, err := wsRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify target branches.
	branches := map[string]string{}
	for _, wr := range results {
		branches[wr.RepoID.String()] = wr.TargetBranch
	}
	if branches[r1.ID.String()] != "feature-a" {
		t.Errorf("repo1 target_branch: got %q, want %q", branches[r1.ID.String()], "feature-a")
	}
	if branches[r2.ID.String()] != "feature-b" {
		t.Errorf("repo2 target_branch: got %q, want %q", branches[r2.ID.String()], "feature-b")
	}

	// Verify workspace_id is correct.
	for _, wr := range results {
		if wr.WorkspaceID != wsID {
			t.Errorf("WorkspaceID: got %s, want %s", wr.WorkspaceID, wsID)
		}
	}
}

func TestWorkspaceRepoRepo_CreateMany_Empty(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	wsRepo := NewWorkspaceRepoRepo(db.DB)
	wsID := createTestWorkspace(t, db.DB)

	// Creating zero entries should succeed without error.
	err := wsRepo.CreateMany(wsID, nil)
	if err != nil {
		t.Fatalf("CreateMany nil: %v", err)
	}

	err = wsRepo.CreateMany(wsID, []domain.CreateWorkspaceRepo{})
	if err != nil {
		t.Fatalf("CreateMany empty: %v", err)
	}
}

func TestWorkspaceRepoRepo_FindByWorkspaceIDWithRepos(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	gitRepo := NewGitRepoRepo(db.DB)
	wsRepo := NewWorkspaceRepoRepo(db.DB)

	r1, err := gitRepo.FindOrCreate("/path/r1", "repo1", "Repo 1")
	if err != nil {
		t.Fatalf("FindOrCreate r1: %v", err)
	}
	r2, err := gitRepo.FindOrCreate("/path/r2", "repo2", "Repo 2")
	if err != nil {
		t.Fatalf("FindOrCreate r2: %v", err)
	}

	wsID := createTestWorkspace(t, db.DB)

	err = wsRepo.CreateMany(wsID, []domain.CreateWorkspaceRepo{
		{RepoID: r1.ID, TargetBranch: "develop"},
		{RepoID: r2.ID, TargetBranch: "main"},
	})
	if err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	results, err := wsRepo.FindByWorkspaceIDWithRepos(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceIDWithRepos: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify each result has repo details + target_branch.
	repoByID := map[string]domain.RepoWithTargetBranch{}
	for _, rwt := range results {
		repoByID[rwt.ID.String()] = rwt
	}

	got1, ok := repoByID[r1.ID.String()]
	if !ok {
		t.Fatal("repo1 not found in results")
	}
	if got1.Path != "/path/r1" {
		t.Errorf("repo1 Path: got %q, want %q", got1.Path, "/path/r1")
	}
	if got1.Name != "repo1" {
		t.Errorf("repo1 Name: got %q, want %q", got1.Name, "repo1")
	}
	if got1.DisplayName != "Repo 1" {
		t.Errorf("repo1 DisplayName: got %q, want %q", got1.DisplayName, "Repo 1")
	}
	if got1.TargetBranch != "develop" {
		t.Errorf("repo1 TargetBranch: got %q, want %q", got1.TargetBranch, "develop")
	}

	got2, ok := repoByID[r2.ID.String()]
	if !ok {
		t.Fatal("repo2 not found in results")
	}
	if got2.TargetBranch != "main" {
		t.Errorf("repo2 TargetBranch: got %q, want %q", got2.TargetBranch, "main")
	}
}

func TestWorkspaceRepoRepo_DeleteByWorkspaceID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	gitRepo := NewGitRepoRepo(db.DB)
	wsRepo := NewWorkspaceRepoRepo(db.DB)

	r1, err := gitRepo.FindOrCreate("/path/r1", "repo1", "Repo 1")
	if err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}

	wsID := createTestWorkspace(t, db.DB)

	err = wsRepo.CreateMany(wsID, []domain.CreateWorkspaceRepo{
		{RepoID: r1.ID, TargetBranch: "feature"},
	})
	if err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	// Verify it was created.
	results, err := wsRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(results))
	}

	// Delete all.
	if err := wsRepo.DeleteByWorkspaceID(wsID); err != nil {
		t.Fatalf("DeleteByWorkspaceID: %v", err)
	}

	// Verify empty.
	results, err = wsRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID after delete: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestWorkspaceRepoRepo_DeleteByWorkspaceAndRepo(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	gitRepo := NewGitRepoRepo(db.DB)
	wsRepo := NewWorkspaceRepoRepo(db.DB)

	r1, err := gitRepo.FindOrCreate("/path/r1", "repo1", "Repo 1")
	if err != nil {
		t.Fatalf("FindOrCreate r1: %v", err)
	}
	r2, err := gitRepo.FindOrCreate("/path/r2", "repo2", "Repo 2")
	if err != nil {
		t.Fatalf("FindOrCreate r2: %v", err)
	}

	wsID := createTestWorkspace(t, db.DB)

	err = wsRepo.CreateMany(wsID, []domain.CreateWorkspaceRepo{
		{RepoID: r1.ID, TargetBranch: "branch-a"},
		{RepoID: r2.ID, TargetBranch: "branch-b"},
	})
	if err != nil {
		t.Fatalf("CreateMany: %v", err)
	}

	// Delete only r1's association.
	if err := wsRepo.DeleteByWorkspaceAndRepo(wsID, r1.ID); err != nil {
		t.Fatalf("DeleteByWorkspaceAndRepo: %v", err)
	}

	// Verify r1 is gone but r2 remains.
	results, err := wsRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after partial delete, got %d", len(results))
	}
	if results[0].RepoID != r2.ID {
		t.Errorf("remaining repo: got %s, want %s", results[0].RepoID, r2.ID)
	}
	if results[0].TargetBranch != "branch-b" {
		t.Errorf("remaining target_branch: got %q, want %q", results[0].TargetBranch, "branch-b")
	}
}
