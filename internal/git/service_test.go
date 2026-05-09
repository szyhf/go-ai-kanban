package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestService_IsRepoOpenable(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	if !svc.IsRepoOpenable(dir) {
		t.Error("expected repo to be openable")
	}

	if svc.IsRepoOpenable(t.TempDir()) {
		t.Error("expected empty dir to not be openable")
	}
}

func TestService_OpenRepo(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	repo, err := svc.OpenRepo(dir)
	if err != nil {
		t.Fatalf("OpenRepo: %v", err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
}

func TestService_GetGitDir(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	gitDir, err := svc.GetGitDir(dir)
	if err != nil {
		t.Fatalf("GetGitDir: %v", err)
	}
	expected := filepath.Join(dir, ".git")
	if gitDir != expected {
		t.Errorf("got %q, want %q", gitDir, expected)
	}
}

func TestService_GetAllBranches(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	branches, err := svc.GetAllBranches(dir)
	if err != nil {
		t.Fatalf("GetAllBranches: %v", err)
	}

	names := make(map[string]bool)
	for _, b := range branches {
		names[b.Name] = true
		if b.Name == "main" && !b.IsCurrent {
			t.Error("main should be current")
		}
		if b.Name == "feature/test" && b.IsCurrent {
			t.Error("feature/test should not be current")
		}
	}

	if !names["main"] {
		t.Error("missing main branch")
	}
	if !names["feature/test"] {
		t.Error("missing feature/test branch")
	}
}

func TestService_GetCurrentBranch(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	branch, err := svc.GetCurrentBranch(dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("got %q, want %q", branch, "main")
	}
}

func TestService_CheckBranchExists(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	exists, err := svc.CheckBranchExists(dir, "main")
	if err != nil {
		t.Fatalf("CheckBranchExists main: %v", err)
	}
	if !exists {
		t.Error("main should exist")
	}

	exists, err = svc.CheckBranchExists(dir, "feature/test")
	if err != nil {
		t.Fatalf("CheckBranchExists feature/test: %v", err)
	}
	if !exists {
		t.Error("feature/test should exist")
	}

	exists, err = svc.CheckBranchExists(dir, "nonexistent")
	if err != nil {
		t.Fatalf("CheckBranchExists nonexistent: %v", err)
	}
	if exists {
		t.Error("nonexistent should not exist")
	}
}

func TestService_GetBranchOID(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	oid, err := svc.GetBranchOID(dir, "main")
	if err != nil {
		t.Fatalf("GetBranchOID: %v", err)
	}
	if len(oid) != 40 {
		t.Errorf("OID length: got %d, want 40", len(oid))
	}
}

func TestService_GetHeadCommit(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	commit := svc.GetHeadCommit(dir)
	if commit == nil {
		t.Fatal("expected non-nil commit")
	}
	if len(commit.Hash) != 40 {
		t.Errorf("Hash length: got %d, want 40", len(commit.Hash))
	}
}

func TestService_GetHeadInfo(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	info, err := svc.GetHeadInfo(dir)
	if err != nil {
		t.Fatalf("GetHeadInfo: %v", err)
	}
	if info.Branch != "main" {
		t.Errorf("Branch: got %q, want %q", info.Branch, "main")
	}
	if len(info.OID) != 40 {
		t.Errorf("OID length: got %d, want 40", len(info.OID))
	}
}

func TestService_IsHeadChildOf(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	head := svc.GetHeadCommit(dir)
	if head == nil {
		t.Fatal("expected head commit")
	}

	if !svc.IsHeadChildOf(dir, head.Hash) {
		t.Error("HEAD should be child of itself")
	}

	if svc.IsHeadChildOf(dir, "0000000000000000000000000000000000000000") {
		t.Error("HEAD should not be child of zero hash")
	}
}

func TestService_CreateBranch(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	if err := svc.CreateBranch(dir, "new-feature", "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	exists, err := svc.CheckBranchExists(dir, "new-feature")
	if err != nil {
		t.Fatalf("CheckBranchExists: %v", err)
	}
	if !exists {
		t.Error("new-feature should exist")
	}
}

func TestService_DeleteBranch(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	if err := svc.DeleteBranch(dir, "feature/test"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	exists, err := svc.CheckBranchExists(dir, "feature/test")
	if err != nil {
		t.Fatalf("CheckBranchExists: %v", err)
	}
	if exists {
		t.Error("feature/test should be deleted")
	}
}

func TestService_IsWorktreeClean(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	clean, err := svc.IsWorktreeClean(dir)
	if err != nil {
		t.Fatalf("IsWorktreeClean: %v", err)
	}
	if !clean {
		t.Error("expected clean worktree")
	}

	// Modify a file.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	clean, err = svc.IsWorktreeClean(dir)
	if err != nil {
		t.Fatalf("IsWorktreeClean dirty: %v", err)
	}
	if clean {
		t.Error("expected dirty worktree")
	}
}

func TestService_Commit(t *testing.T) {
	dir := setupTestRepo(t)
	svc := NewService()

	// No changes — should return false.
	didCommit, err := svc.Commit(dir, "no-op commit")
	if err != nil {
		t.Fatalf("Commit no-op: %v", err)
	}
	if didCommit {
		t.Error("expected false for no changes")
	}

	// Add a change.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	didCommit, err = svc.Commit(dir, "Add new file")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !didCommit {
		t.Error("expected true for actual commit")
	}

	// Verify clean after commit.
	clean, err := svc.IsWorktreeClean(dir)
	if err != nil {
		t.Fatalf("IsWorktreeClean: %v", err)
	}
	if !clean {
		t.Error("expected clean after commit")
	}
}

func TestService_GetBaseCommit(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	base, err := svc.GetBaseCommit(dir, "feature/test", "main")
	if err != nil {
		t.Fatalf("GetBaseCommit: %v", err)
	}
	if base == nil {
		t.Fatal("expected non-nil base commit")
	}
	if len(base.Hash) != 40 {
		t.Errorf("Hash length: got %d, want 40", len(base.Hash))
	}

	// The base commit should be the main branch HEAD.
	mainHead := svc.GetHeadCommit(dir)
	if mainHead == nil {
		t.Fatal("expected main head commit")
	}
	if base.Hash != mainHead.Hash {
		t.Errorf("base = %s, main HEAD = %s, expected equal", base.Hash, mainHead.Hash)
	}
}

func TestService_GetBranchStatus(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	ahead, behind, err := svc.GetBranchStatus(dir, "feature/test", "main")
	if err != nil {
		t.Fatalf("GetBranchStatus: %v", err)
	}
	if ahead != 1 {
		t.Errorf("ahead: got %d, want 1", ahead)
	}
	if behind != 0 {
		t.Errorf("behind: got %d, want 0", behind)
	}
}

func TestService_GetDiffs(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	mainHead := svc.GetHeadCommit(dir)
	if mainHead == nil {
		t.Fatal("expected head commit")
	}

	// Switch to feature branch.
	runGit(t, dir, "checkout", "feature/test")

	diffs, err := svc.GetDiffs(dir, mainHead, nil, "repo-123")
	if err != nil {
		t.Fatalf("GetDiffs: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	d := diffs[0]
	if d.RepoID != "repo-123" {
		t.Errorf("RepoID: got %q, want %q", d.RepoID, "repo-123")
	}
	if d.Change != DiffChangeAdded {
		t.Errorf("Change: got %q, want %q", d.Change, DiffChangeAdded)
	}
	if d.NewPath != "feature.txt" {
		t.Errorf("NewPath: got %q, want %q", d.NewPath, "feature.txt")
	}
}

func TestService_GetDiffFilePaths(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	mainHead := svc.GetHeadCommit(dir)
	if mainHead == nil {
		t.Fatal("expected head commit")
	}

	runGit(t, dir, "checkout", "feature/test")

	paths, err := svc.GetDiffFilePaths(dir, mainHead)
	if err != nil {
		t.Fatalf("GetDiffFilePaths: %v", err)
	}

	if !paths["feature.txt"] {
		t.Error("expected feature.txt in diff paths")
	}
}

func TestService_IsBranchNameValid(t *testing.T) {
	svc := NewService()

	if !svc.IsBranchNameValid("main") {
		t.Error("main should be valid")
	}
	if svc.IsBranchNameValid("test..name") {
		t.Error("test..name should be invalid")
	}
}

func TestService_InitializeRepoWithMainBranch(t *testing.T) {
	dir := t.TempDir()
	svc := NewService()

	if err := svc.InitializeRepoWithMainBranch(dir); err != nil {
		t.Fatalf("InitializeRepoWithMainBranch: %v", err)
	}

	if !svc.IsRepoOpenable(dir) {
		t.Error("expected repo to be openable after init")
	}

	branch, err := svc.GetCurrentBranch(dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("got %q, want %q", branch, "main")
	}
}

func TestService_RenameLocalBranch(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	svc := NewService()

	// Checkout feature/test first.
	runGit(t, dir, "checkout", "feature/test")

	if err := svc.RenameLocalBranch(dir, "feature/test", "feature/renamed"); err != nil {
		t.Fatalf("RenameLocalBranch: %v", err)
	}

	current, err := svc.GetCurrentBranch(dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch: %v", err)
	}
	if current != "feature/renamed" {
		t.Errorf("got %q, want %q", current, "feature/renamed")
	}
}
