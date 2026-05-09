package service

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/git"
)

func setupFilesystemService(t *testing.T) *FilesystemService {
	t.Helper()
	return NewFilesystemService(git.NewService())
}

func setupNestedGitRepos(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitSvc := git.NewService()

	// Create two git repos under dir.
	repo1 := filepath.Join(dir, "project-a")
	os.MkdirAll(repo1, 0o755)
	if err := gitSvc.InitializeRepoWithMainBranch(repo1); err != nil {
		t.Fatalf("init repo1: %v", err)
	}

	repo2 := filepath.Join(dir, "project-b")
	os.MkdirAll(repo2, 0o755)
	if err := gitSvc.InitializeRepoWithMainBranch(repo2); err != nil {
		t.Fatalf("init repo2: %v", err)
	}

	// Create a non-git directory.
	os.MkdirAll(filepath.Join(dir, "plain-dir"), 0o755)

	// Create a nested git repo.
	nested := filepath.Join(dir, "workspace", "deep-project")
	os.MkdirAll(nested, 0o755)
	if err := gitSvc.InitializeRepoWithMainBranch(nested); err != nil {
		t.Fatalf("init nested repo: %v", err)
	}

	return dir
}

func TestFilesystemService_ListGitRepos(t *testing.T) {
	svc := setupFilesystemService(t)
	base := setupNestedGitRepos(t)

	repos, err := svc.ListGitRepos(base, 5)
	if err != nil {
		t.Fatalf("ListGitRepos: %v", err)
	}

	// Should find 3 repos.
	if len(repos) < 3 {
		t.Errorf("expected at least 3 repos, got %d", len(repos))
	}

	// All results should be git repos.
	for _, r := range repos {
		if !r.IsGitRepo || !r.IsDirectory {
			t.Errorf("expected git repo directory, got %+v", r)
		}
	}
}

func TestFilesystemService_ListGitReposMaxDepth(t *testing.T) {
	svc := setupFilesystemService(t)
	base := setupNestedGitRepos(t)

	// Max depth 1: should not find the nested deep-project.
	repos, err := svc.ListGitRepos(base, 1)
	if err != nil {
		t.Fatalf("ListGitRepos: %v", err)
	}

	for _, r := range repos {
		if filepath.Base(r.Path) == "deep-project" {
			t.Error("deep-project should not be found with maxDepth=1")
		}
	}
}

func TestFilesystemService_ListGitReposNonExistent(t *testing.T) {
	svc := setupFilesystemService(t)

	_, err := svc.ListGitRepos("/nonexistent/path", 5)
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestFilesystemService_ListDirectory(t *testing.T) {
	svc := setupFilesystemService(t)
	dir := t.TempDir()

	// Create entries.
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0o644)
	gitSvc := git.NewService()
	repoDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(repoDir, 0o755)
	gitSvc.InitializeRepoWithMainBranch(repoDir)

	resp, err := svc.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}

	if resp.CurrentPath != dir {
		t.Errorf("CurrentPath: got %q, want %q", resp.CurrentPath, dir)
	}

	names := make([]string, len(resp.Entries))
	for i, e := range resp.Entries {
		names[i] = e.Name
	}
	sort.Strings(names)

	// Should have subdir, file.txt, myrepo. Hidden entries like .git are skipped.
	expected := []string{"file.txt", "myrepo", "subdir"}
	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("missing entry %q in %v", e, names)
		}
	}

	// Verify myrepo is detected as git repo.
	for _, e := range resp.Entries {
		if e.Name == "myrepo" {
			if !e.IsGitRepo {
				t.Error("myrepo should be detected as git repo")
			}
		}
	}
}

func TestFilesystemService_ListDirectorySkipsHidden(t *testing.T) {
	svc := setupFilesystemService(t)
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	os.WriteFile(filepath.Join(dir, ".hidden_file"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(dir, "visible"), 0o755)

	resp, err := svc.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}

	for _, e := range resp.Entries {
		if e.Name[0] == '.' {
			t.Errorf("hidden entry should be skipped: %q", e.Name)
		}
	}
}

func TestGetDirectoriesToSkip(t *testing.T) {
	skip := getDirectoriesToSkip()

	expected := []string{"node_modules", "target", "build", "dist", "Library", "Applications"}
	for _, name := range expected {
		if !skip[name] {
			t.Errorf("expected %q in skip set", name)
		}
	}
}
