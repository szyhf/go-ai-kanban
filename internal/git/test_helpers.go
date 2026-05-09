package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository with an initial commit
// and returns the path. The repository is cleaned up automatically.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Init repo.
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")

	// Create an initial file and commit.
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Initial commit")

	return dir
}

// setupTestRepoWithBranches creates a repo with multiple branches.
func setupTestRepoWithBranches(t *testing.T) string {
	t.Helper()

	dir := setupTestRepo(t)

	// Create a feature branch with a commit.
	runGit(t, dir, "checkout", "-b", "feature/test")
	feature := filepath.Join(dir, "feature.txt")
	if err := os.WriteFile(feature, []byte("feature content\n"), 0o644); err != nil {
		t.Fatalf("write feature: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Add feature")

	// Switch back to main.
	runGit(t, dir, "checkout", "main")

	return dir
}

// runGit executes a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2025-01-01T00:00:00+00:00", "GIT_COMMITTER_DATE=2025-01-01T00:00:00+00:00")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// getHeadSHA returns the current HEAD commit SHA.
func getHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return string(out[:40])
}
