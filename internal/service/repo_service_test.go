package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/git"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
)

func setupServiceTestDB(t *testing.T) *repository.GitRepoRepo {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := database.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return repository.NewGitRepoRepo(db.DB)
}

func setupRepoService(t *testing.T) *RepoService {
	t.Helper()
	repoRepo := setupServiceTestDB(t)
	gitSvc := git.NewService()
	return NewRepoService(repoRepo, gitSvc)
}

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitSvc := git.NewService()
	if err := gitSvc.InitializeRepoWithMainBranch(dir); err != nil {
		t.Fatalf("InitializeRepoWithMainBranch: %v", err)
	}
	return dir
}

func TestRepoService_NormalizePath(t *testing.T) {
	svc := NewRepoService(nil, nil)

	// Test absolute path.
	abs, err := svc.NormalizePath("/tmp/test")
	if err != nil {
		t.Fatalf("NormalizePath: %v", err)
	}
	if abs != "/tmp/test" {
		t.Errorf("got %q, want /tmp/test", abs)
	}

	// Test tilde expansion.
	home, _ := os.UserHomeDir()
	expanded, err := svc.NormalizePath("~/projects")
	if err != nil {
		t.Fatalf("NormalizePath ~: %v", err)
	}
	if expanded != filepath.Join(home, "projects") {
		t.Errorf("got %q, want %q", expanded, filepath.Join(home, "projects"))
	}
}

func TestRepoService_Register(t *testing.T) {
	svc := setupRepoService(t)
	repoDir := setupTestGitRepo(t)

	repo, err := svc.Register(repoDir, "My Repo")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if repo == nil {
		t.Fatal("expected non-nil repo")
	}
	if repo.DisplayName != "My Repo" {
		t.Errorf("DisplayName: got %q, want %q", repo.DisplayName, "My Repo")
	}
	if repo.Path != repoDir {
		t.Errorf("Path: got %q, want %q", repo.Path, repoDir)
	}

	// Register again (idempotent).
	repo2, err := svc.Register(repoDir, "My Repo")
	if err != nil {
		t.Fatalf("Register again: %v", err)
	}
	if repo2.ID != repo.ID {
		t.Error("expected same ID for duplicate registration")
	}
}

func TestRepoService_RegisterDefaultName(t *testing.T) {
	svc := setupRepoService(t)
	repoDir := setupTestGitRepo(t)

	repo, err := svc.Register(repoDir, "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	expectedName := filepath.Base(repoDir)
	if repo.DisplayName != expectedName {
		t.Errorf("DisplayName: got %q, want %q", repo.DisplayName, expectedName)
	}
}

func TestRepoService_RegisterNotGitRepo(t *testing.T) {
	svc := setupRepoService(t)

	_, err := svc.Register(t.TempDir(), "test")
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestRepoService_FindByID(t *testing.T) {
	svc := setupRepoService(t)
	repoDir := setupTestGitRepo(t)

	registered, err := svc.Register(repoDir, "test")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	found, err := svc.FindByID(registered.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil || found.ID != registered.ID {
		t.Error("FindByID mismatch")
	}

	// Not found.
	notFound, err := svc.FindByID(domain.NewUUID())
	if err != nil {
		t.Fatalf("FindByID not found: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent ID")
	}
}

func TestRepoService_GetByID(t *testing.T) {
	svc := setupRepoService(t)

	_, err := svc.GetByID(domain.NewUUID())
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestRepoService_InitRepo(t *testing.T) {
	svc := setupRepoService(t)
	parentDir := t.TempDir()

	repo, err := svc.InitRepo(parentDir, "my-project")
	if err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	if repo.DisplayName != "my-project" {
		t.Errorf("DisplayName: got %q, want %q", repo.DisplayName, "my-project")
	}

	// Verify it's a valid git repo.
	if !git.NewService().IsRepoOpenable(filepath.Join(parentDir, "my-project")) {
		t.Error("expected initialized repo to be openable")
	}
}

func TestRepoService_InitRepoInvalidNames(t *testing.T) {
	svc := setupRepoService(t)
	parentDir := t.TempDir()

	tests := []struct {
		name   string
		input  string
		errMsg string
	}{
		{"empty", "", "empty"},
		{"dot", ".", "invalid"},
		{"dotdot", "..", "invalid"},
		{"slash", "foo/bar", "slashes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.InitRepo(parentDir, tt.input)
			if err == nil {
				t.Errorf("expected error for %q", tt.input)
			}
		})
	}
}

func TestRepoService_SearchFiles(t *testing.T) {
	svc := setupRepoService(t)
	repoDir := setupTestGitRepo(t)

	// Create some files.
	os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(repoDir, "helper.go"), []byte("package main"), 0o644)
	os.MkdirAll(filepath.Join(repoDir, "src"), 0o755)
	os.WriteFile(filepath.Join(repoDir, "src", "app.ts"), []byte("export {}"), 0o644)

	repos := []domain.Repo{{Path: repoDir, Name: filepath.Base(repoDir)}}

	results, err := svc.SearchFiles(repos, "main.go")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	// First result should be the exact filename match.
	if results[0].MatchType != domain.SearchMatchFileName {
		t.Errorf("expected FileName match, got %s", results[0].MatchType)
	}
}
