package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLI_HasChanges(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	has, err := cli.HasChanges(dir)
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if has {
		t.Error("expected no changes after initial commit")
	}

	// Modify a file.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	has, err = cli.HasChanges(dir)
	if err != nil {
		t.Fatalf("HasChanges after modify: %v", err)
	}
	if !has {
		t.Error("expected changes after modification")
	}
}

func TestCLI_AddAll_Commit(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	// Add a new file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new file\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := cli.AddAll(dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}

	if err := cli.Commit(dir, "Add new file"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify no changes after commit.
	has, err := cli.HasChanges(dir)
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if has {
		t.Error("expected no changes after commit")
	}
}

func TestCLI_GetWorktreeStatus(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	// Clean worktree.
	status, err := cli.GetWorktreeStatus(dir)
	if err != nil {
		t.Fatalf("GetWorktreeStatus: %v", err)
	}
	if status.UncommittedTracked != 0 {
		t.Errorf("UncommittedTracked: got %d, want 0", status.UncommittedTracked)
	}
	if status.Untracked != 0 {
		t.Errorf("Untracked: got %d, want 0", status.Untracked)
	}

	// Add untracked file.
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	status, err = cli.GetWorktreeStatus(dir)
	if err != nil {
		t.Fatalf("GetWorktreeStatus untracked: %v", err)
	}
	if status.Untracked != 1 {
		t.Errorf("Untracked: got %d, want 1", status.Untracked)
	}
}

func TestCLI_DeleteBranch(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	cli := NewCLI()

	if err := cli.DeleteBranch(dir, "feature/test"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
}

func TestCLI_ListRemotes(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	// No remotes by default.
	remotes, err := cli.ListRemotes(dir)
	if err != nil {
		t.Fatalf("ListRemotes: %v", err)
	}
	if len(remotes) != 0 {
		t.Errorf("expected 0 remotes, got %d", len(remotes))
	}

	// Add a remote.
	runGit(t, dir, "remote", "add", "origin", "https://github.com/test/repo.git")

	remotes, err = cli.ListRemotes(dir)
	if err != nil {
		t.Fatalf("ListRemotes with origin: %v", err)
	}
	if len(remotes) != 1 {
		t.Fatalf("expected 1 remote, got %d", len(remotes))
	}
	if remotes[0].Name != "origin" {
		t.Errorf("Name: got %q, want %q", remotes[0].Name, "origin")
	}
}

func TestCLI_GetRemoteURL(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	runGit(t, dir, "remote", "add", "origin", "https://github.com/test/repo.git")

	url, err := cli.GetRemoteURL(dir, "origin")
	if err != nil {
		t.Fatalf("GetRemoteURL: %v", err)
	}
	if url != "https://github.com/test/repo.git" {
		t.Errorf("got %q, want %q", url, "https://github.com/test/repo.git")
	}
}

func TestCLI_UpdateRef(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	sha := getHeadSHA(t, dir)

	// Create a new branch and update its ref.
	runGit(t, dir, "branch", "test-branch")

	newSHA := sha // Same SHA for simplicity.
	if err := cli.UpdateRef(dir, "refs/heads/test-branch", newSHA); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}
}

func TestCLI_WorktreeOperations(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	cli := NewCLI()

	// List worktrees (should be 1 — main).
	entries, err := cli.ListWorktrees(dir)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(entries) < 1 {
		t.Fatalf("expected at least 1 worktree, got %d", len(entries))
	}

	// Add a worktree.
	wtPath := filepath.Join(t.TempDir(), "wt-test")
	if err := cli.WorktreeAdd(dir, wtPath, "feature/test", false); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(wtPath) })

	// Verify it appears in list.
	entries, err = cli.ListWorktrees(dir)
	if err != nil {
		t.Fatalf("ListWorktrees after add: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Branch == "feature/test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("worktree for feature/test not found in list")
	}

	// Remove the worktree.
	if err := cli.WorktreeRemove(dir, wtPath, true); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
}

func TestCLI_IsRebaseInProgress(t *testing.T) {
	dir := setupTestRepo(t)
	cli := NewCLI()

	inProgress, err := cli.IsRebaseInProgress(dir)
	if err != nil {
		t.Fatalf("IsRebaseInProgress: %v", err)
	}
	if inProgress {
		t.Error("expected no rebase in progress")
	}
}

func TestCLI_DiffStatus(t *testing.T) {
	dir := setupTestRepoWithBranches(t)
	cli := NewCLI()

	headSHA := getHeadSHA(t, dir)

	// Modify a file on main.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := cli.DiffStatus(dir, headSHA, nil)
	if err != nil {
		t.Fatalf("DiffStatus: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 diff entry, got %d", len(entries))
	}
	if entries[0].Change != ChangeModified {
		t.Errorf("Change: got %q, want %q", entries[0].Change, ChangeModified)
	}
	if entries[0].Path != "README.md" {
		t.Errorf("Path: got %q, want %q", entries[0].Path, "README.md")
	}
}

func TestParseNameStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   []StatusDiffEntry
	}{
		{
			"empty",
			"",
			nil,
		},
		{
			"single_modified",
			"M\tfile.txt\n",
			[]StatusDiffEntry{{Change: ChangeModified, Path: "file.txt"}},
		},
		{
			"single_added",
			"A\tnew.txt\n",
			[]StatusDiffEntry{{Change: ChangeAdded, Path: "new.txt"}},
		},
		{
			"rename_with_score",
			"R100\told.txt\tnew.txt\n",
			[]StatusDiffEntry{{Change: ChangeRenamed, OldPath: "old.txt", Path: "new.txt"}},
		},
		{
			"multiple",
			"A\tadded.txt\nD\tdeleted.txt\nM\tmodified.txt\n",
			[]StatusDiffEntry{
				{Change: ChangeAdded, Path: "added.txt"},
				{Change: ChangeDeleted, Path: "deleted.txt"},
				{Change: ChangeModified, Path: "modified.txt"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNameStatus(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for i, g := range got {
				w := tt.want[i]
				if g.Change != w.Change || g.Path != w.Path || g.OldPath != w.OldPath {
					t.Errorf("entry[%d]: got {Change:%s, Path:%s, OldPath:%s}, want {Change:%s, Path:%s, OldPath:%s}",
						i, g.Change, g.Path, g.OldPath, w.Change, w.Path, w.OldPath)
				}
			}
		})
	}
}
