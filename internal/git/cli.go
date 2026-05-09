package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CLI wraps the git binary for working-tree mutations.
// All commands run with `git -C <path>` to set the working directory.
type CLI struct{}

// NewCLI creates a new CLI instance.
func NewCLI() *CLI { return &CLI{} }

// --- Worktree management ---

// WorktreeAdd creates a new worktree at worktreePath for the given branch.
// If createBranch is true, a new branch is created.
func (c *CLI) WorktreeAdd(repoPath, worktreePath, branch string, createBranch bool) error {
	args := []string{"worktree", "add"}
	if createBranch {
		args = append(args, "-b", branch)
	}
	args = append(args, worktreePath)
	if !createBranch {
		args = append(args, branch)
	}
	_, err := c.run(repoPath, args)
	if err != nil {
		return fmt.Errorf("worktree add: %w", err)
	}

	// Reapply sparse-checkout in the new worktree (non-fatal).
	_ = c.SparseCheckoutReapply(worktreePath)
	return nil
}

// WorktreeRemove removes a worktree. If force is true, forces removal.
func (c *CLI) WorktreeRemove(repoPath, worktreePath string, force bool) error {
	args := []string{"worktree", "remove", worktreePath}
	if force {
		args = append(args, "--force")
	}
	_, err := c.run(repoPath, args)
	if err != nil {
		return fmt.Errorf("worktree remove: %w", err)
	}
	return nil
}

// WorktreeMove moves a worktree from oldPath to newPath.
func (c *CLI) WorktreeMove(repoPath, oldPath, newPath string) error {
	_, err := c.run(repoPath, []string{"worktree", "move", oldPath, newPath})
	if err != nil {
		return fmt.Errorf("worktree move: %w", err)
	}
	return nil
}

// WorktreePrune prunes stale worktree metadata.
func (c *CLI) WorktreePrune(repoPath string) error {
	_, err := c.run(repoPath, []string{"worktree", "prune"})
	if err != nil {
		return fmt.Errorf("worktree prune: %w", err)
	}
	return nil
}

// WorktreeEntry represents a parsed worktree from `git worktree list --porcelain`.
type WorktreeEntry struct {
	Path   string
	Branch string // empty for bare/main worktree
}

// ListWorktrees returns all worktrees for a repository.
func (c *CLI) ListWorktrees(repoPath string) ([]WorktreeEntry, error) {
	out, err := c.run(repoPath, []string{"worktree", "list", "--porcelain"})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	var entries []WorktreeEntry
	var current *WorktreeEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				entries = append(entries, *current)
				current = nil
			}
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		switch parts[0] {
		case "worktree":
			current = &WorktreeEntry{Path: parts[1]}
		case "branch":
			if current != nil {
				// Strip refs/heads/ prefix.
				current.Branch = strings.TrimPrefix(parts[1], "refs/heads/")
			}
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries, nil
}

// --- Status / diff ---

// StatusEntry represents a single entry from `git status --porcelain -z`.
type StatusEntry struct {
	Staged     byte   // Column X
	Unstaged   byte   // Column Y
	Path       string // File path (UTF-8)
	OrigPath   string // Original path for renames
	IsUntracked bool
}

// WorktreeStatus represents the status of a worktree.
type WorktreeStatus struct {
	UncommittedTracked int
	Untracked          int
	Entries            []StatusEntry
}

// GetWorktreeStatus returns the full status of a worktree.
func (c *CLI) GetWorktreeStatus(worktreePath string) (*WorktreeStatus, error) {
	out, err := c.runRaw(worktreePath, []string{"status", "--porcelain", "-z"})
	if err != nil {
		return nil, fmt.Errorf("get worktree status: %w", err)
	}

	status := &WorktreeStatus{}
	if len(out) == 0 {
		return status, nil
	}

	// Parse NUL-separated output: XY PATH\0[ORIG_PATH\0]
	entries := bytes.Split(out, []byte{0})
	for i := 0; i < len(entries); {
		if len(entries[i]) == 0 {
			i++
			continue
		}
		entry := string(entries[i])
		if len(entry) < 3 {
			i++
			continue
		}

		se := StatusEntry{
			Staged:   entry[0],
			Unstaged: entry[1],
		}
		se.Path = entry[3:] // skip "XY "

		// Determine untracked.
		if se.Staged == '?' && se.Unstaged == '?' {
			se.IsUntracked = true
			status.Untracked++
		} else {
			status.UncommittedTracked++
		}

		// Check for rename original (next entry is the original path).
		if (se.Staged == 'R' || se.Unstaged == 'R') && i+1 < len(entries) {
			i++
			if len(entries[i]) > 0 {
				se.OrigPath = string(entries[i])
			}
		}

		status.Entries = append(status.Entries, se)
		i++
	}
	return status, nil
}

// HasChanges returns true if the worktree has any changes.
func (c *CLI) HasChanges(worktreePath string) (bool, error) {
	out, err := c.run(worktreePath, []string{"status", "--porcelain"})
	if err != nil {
		return false, fmt.Errorf("has changes: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// ChangeType represents the type of change in a diff.
type ChangeType string

const (
	ChangeAdded      ChangeType = "added"
	ChangeModified   ChangeType = "modified"
	ChangeDeleted    ChangeType = "deleted"
	ChangeRenamed    ChangeType = "renamed"
	ChangeCopied     ChangeType = "copied"
	ChangeTypeChanged ChangeType = "type_changed"
	ChangeUnmerged   ChangeType = "unmerged"
	ChangeUnknown    ChangeType = "unknown"
)

// StatusDiffEntry represents one entry from a diff status.
type StatusDiffEntry struct {
	Change  ChangeType
	Path    string
	OldPath string // Set for renames/copies
}

// DiffStatus returns the diff status between the worktree and a base commit.
// It uses a temporary index to avoid modifying the real index.
func (c *CLI) DiffStatus(worktreePath string, baseCommit string, pathFilter []string) ([]StatusDiffEntry, error) {
	// Create a temporary directory for the index.
	tmpDir, err := os.MkdirTemp("", "git-diff-index-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpIndex := filepath.Join(tmpDir, "index")

	args := []string{
		"read-tree", baseCommit,
	}
	_, err = c.runWithEnv(worktreePath, args, []string{"GIT_INDEX_FILE=" + tmpIndex})
	if err != nil {
		return nil, fmt.Errorf("read-tree: %w", err)
	}

	args = []string{
		"add", "-A",
	}
	if len(pathFilter) > 0 {
		args = append(args, "--")
		args = append(args, pathFilter...)
	}
	_, err = c.runWithEnv(worktreePath, args, []string{"GIT_INDEX_FILE=" + tmpIndex})
	if err != nil {
		return nil, fmt.Errorf("add: %w", err)
	}

	args = []string{
		"diff", "--cached", "--name-status", baseCommit,
	}
	args = append(args, c.pathspecExcludes()...)
	if len(pathFilter) > 0 {
		args = append(args, "--")
		args = append(args, pathFilter...)
	}
	out, err := c.runWithEnv(worktreePath, args, []string{"GIT_INDEX_FILE=" + tmpIndex})
	if err != nil {
		return nil, fmt.Errorf("diff cached: %w", err)
	}

	return parseNameStatus(out), nil
}

// parseNameStatus parses output of `git diff --name-status`.
func parseNameStatus(output string) []StatusDiffEntry {
	var entries []StatusDiffEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		// Strip rename score, e.g. "R100" -> "R".
		if len(status) > 1 {
			status = status[:1]
		}

		entry := StatusDiffEntry{
			Path: parts[1],
		}
		switch status {
		case "A":
			entry.Change = ChangeAdded
		case "M":
			entry.Change = ChangeModified
		case "D":
			entry.Change = ChangeDeleted
		case "R":
			entry.Change = ChangeRenamed
			if len(parts) >= 3 {
				entry.OldPath = parts[1]
				entry.Path = parts[2]
			}
		case "C":
			entry.Change = ChangeCopied
			if len(parts) >= 3 {
				entry.OldPath = parts[1]
				entry.Path = parts[2]
			}
		case "T":
			entry.Change = ChangeTypeChanged
		case "U":
			entry.Change = ChangeUnmerged
		default:
			entry.Change = ChangeUnknown
		}
		entries = append(entries, entry)
	}
	return entries
}

// --- Staging / committing ---

// AddAll stages all changes in the worktree.
func (c *CLI) AddAll(worktreePath string) error {
	_, err := c.run(worktreePath, []string{"add", "-A"})
	if err != nil {
		return fmt.Errorf("add all: %w", err)
	}
	return nil
}

// Commit creates a commit with the given message.
func (c *CLI) Commit(worktreePath, message string) error {
	_, err := c.run(worktreePath, []string{"commit", "-m", message})
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// --- Merge / rebase ---

// MergeSquashCommit performs a squash merge from fromBranch into the current
// branch and commits with the given message. Returns the new HEAD SHA.
func (c *CLI) MergeSquashCommit(repoPath, baseBranch, fromBranch, message string) (string, error) {
	// Perform squash merge.
	_, err := c.run(repoPath, []string{"merge", "--squash", fromBranch})
	if err != nil {
		// Try to abort the merge on failure.
		_ = c.AbortMerge(repoPath)
		return "", fmt.Errorf("merge squash: %w", err)
	}

	// Stage and commit.
	if err := c.AddAll(repoPath); err != nil {
		return "", fmt.Errorf("add all after squash: %w", err)
	}
	if err := c.Commit(repoPath, message); err != nil {
		return "", fmt.Errorf("commit squash merge: %w", err)
	}

	// Get new HEAD SHA.
	out, err := c.run(repoPath, []string{"rev-parse", "HEAD"})
	if err != nil {
		return "", fmt.Errorf("rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// RebaseOnto rebases taskBranch from oldBase onto newBase.
func (c *CLI) RebaseOnto(worktreePath, newBase, oldBase, taskBranch string) error {
	_, err := c.run(worktreePath, []string{"rebase", "--onto", newBase, oldBase, taskBranch})
	if err != nil {
		return fmt.Errorf("rebase onto: %w", err)
	}
	return nil
}

// AbortRebase aborts an in-progress rebase.
func (c *CLI) AbortRebase(worktreePath string) error {
	_, err := c.run(worktreePath, []string{"rebase", "--abort"})
	if err != nil {
		return fmt.Errorf("rebase abort: %w", err)
	}
	return nil
}

// ContinueRebase continues an in-progress rebase.
func (c *CLI) ContinueRebase(worktreePath string) error {
	_, err := c.runWithEnv(worktreePath, []string{"rebase", "--continue"}, []string{"GIT_EDITOR=true"})
	if err != nil {
		return fmt.Errorf("rebase continue: %w", err)
	}
	return nil
}

// AbortMerge aborts an in-progress merge.
func (c *CLI) AbortMerge(worktreePath string) error {
	_, err := c.run(worktreePath, []string{"merge", "--abort"})
	if err != nil {
		return fmt.Errorf("merge abort: %w", err)
	}
	return nil
}

// AbortCherryPick aborts an in-progress cherry-pick.
func (c *CLI) AbortCherryPick(worktreePath string) error {
	_, err := c.run(worktreePath, []string{"cherry-pick", "--abort"})
	if err != nil {
		return fmt.Errorf("cherry-pick abort: %w", err)
	}
	return nil
}

// AbortRevert aborts an in-progress revert.
func (c *CLI) AbortRevert(worktreePath string) error {
	_, err := c.run(worktreePath, []string{"revert", "--abort"})
	if err != nil {
		return fmt.Errorf("revert abort: %w", err)
	}
	return nil
}

// --- Remote / network ---

// Push pushes a branch to the remote. If force is true, forces the push.
func (c *CLI) Push(worktreePath, remoteURL, branch string, force bool) error {
	refspec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
	if force {
		refspec = "+" + refspec
	}
	_, err := c.runWithEnv(worktreePath, []string{"push", remoteURL, refspec},
		[]string{"GIT_TERMINAL_PROMPT=0"})
	if err != nil {
		return classifyPushError(err)
	}
	return nil
}

// FetchWithRefspec fetches a refspec from a remote URL.
func (c *CLI) FetchWithRefspec(repoPath, remoteURL, refspec string) error {
	_, err := c.runWithEnv(repoPath, []string{"fetch", remoteURL, refspec},
		[]string{"GIT_TERMINAL_PROMPT=0"})
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	return nil
}

// CheckRemoteBranchExists checks whether a branch exists on a remote.
func (c *CLI) CheckRemoteBranchExists(repoPath, remoteURL, branch string) (bool, error) {
	ref := "refs/heads/" + branch
	_, err := c.runWithEnv(repoPath, []string{"ls-remote", "--heads", remoteURL, ref},
		[]string{"GIT_TERMINAL_PROMPT=0"})
	if err != nil {
		return false, classifyAuthError(err)
	}
	return true, nil
}

// ListRemotes returns all remotes for a repository.
func (c *CLI) ListRemotes(repoPath string) ([]GitRemote, error) {
	out, err := c.run(repoPath, []string{"remote", "-v"})
	if err != nil {
		return nil, fmt.Errorf("list remotes: %w", err)
	}

	seen := make(map[string]bool)
	var remotes []GitRemote
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "name\turl (fetch)" or "name\turl (push)"
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		url := strings.TrimSuffix(parts[1], " (fetch)")
		url = strings.TrimSuffix(url, " (push)")
		url = strings.TrimSpace(url)

		if !seen[name] {
			seen[name] = true
			remotes = append(remotes, GitRemote{Name: name, URL: url})
		}
	}
	return remotes, nil
}

// GetRemoteURL returns the URL of a named remote.
func (c *CLI) GetRemoteURL(repoPath, remoteName string) (string, error) {
	out, err := c.run(repoPath, []string{"remote", "get-url", remoteName})
	if err != nil {
		return "", fmt.Errorf("get remote url: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// --- Conflict detection ---

// GetConflictedFiles returns the list of files with merge conflicts.
func (c *CLI) GetConflictedFiles(worktreePath string) ([]string, error) {
	out, err := c.run(worktreePath, []string{"diff", "--name-only", "--diff-filter=U"})
	if err != nil {
		return nil, fmt.Errorf("get conflicted files: %w", err)
	}

	var files []string
	for _, f := range strings.Split(out, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// DetectConflictOp determines what operation caused the current conflict.
func (c *CLI) DetectConflictOp(worktreePath string) (ConflictOp, bool, error) {
	gitDir, err := c.getGitDir(worktreePath)
	if err != nil {
		return "", false, err
	}

	if fileExists(filepath.Join(gitDir, "rebase-merge")) || fileExists(filepath.Join(gitDir, "rebase-apply")) {
		return ConflictOpRebase, true, nil
	}
	if fileExists(filepath.Join(gitDir, "MERGE_HEAD")) {
		return ConflictOpMerge, true, nil
	}
	if fileExists(filepath.Join(gitDir, "CHERRY_PICK_HEAD")) {
		return ConflictOpCherryPick, true, nil
	}
	if fileExists(filepath.Join(gitDir, "REVERT_HEAD")) {
		return ConflictOpRevert, true, nil
	}
	return "", false, nil
}

// IsRebaseInProgress checks if a rebase is in progress.
func (c *CLI) IsRebaseInProgress(worktreePath string) (bool, error) {
	gitDir, err := c.getGitDir(worktreePath)
	if err != nil {
		return false, err
	}
	return fileExists(filepath.Join(gitDir, "rebase-merge")) ||
		fileExists(filepath.Join(gitDir, "rebase-apply")), nil
}

// --- Ref operations ---

// UpdateRef updates a ref to point to a new SHA.
func (c *CLI) UpdateRef(repoPath, refname, sha string) error {
	_, err := c.run(repoPath, []string{"update-ref", refname, sha})
	if err != nil {
		return fmt.Errorf("update ref: %w", err)
	}
	return nil
}

// DeleteBranch force-deletes a branch.
func (c *CLI) DeleteBranch(repoPath, branch string) error {
	_, err := c.run(repoPath, []string{"branch", "-D", branch})
	if err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	return nil
}

// ResetHard resets the worktree to the given commit.
func (c *CLI) ResetHard(worktreePath, commitSHA string) error {
	_, err := c.run(worktreePath, []string{"reset", "--hard", commitSHA})
	if err != nil {
		return fmt.Errorf("reset hard: %w", err)
	}
	return nil
}

// SparseCheckoutReapply reapplies sparse-checkout in a worktree (non-fatal).
func (c *CLI) SparseCheckoutReapply(worktreePath string) error {
	_, err := c.run(worktreePath, []string{"sparse-checkout", "reapply"})
	if err != nil {
		// Non-fatal: sparse-checkout may not be configured.
		return nil
	}
	return nil
}

// --- Private helpers ---

// run executes a git command and returns stdout as a string.
func (c *CLI) run(repoPath string, args []string) (string, error) {
	return c.runWithEnv(repoPath, args, nil)
}

// runWithEnv executes a git command with additional environment variables.
func (c *CLI) runWithEnv(repoPath string, args []string, envs []string) (string, error) {
	out, err := c.runRaw(repoPath, args)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// runRaw executes a git command and returns raw stdout bytes.
func (c *CLI) runRaw(repoPath string, args []string, envs ...[]string) ([]byte, error) {
	fullArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", fullArgs...)

	// Combine custom envs with current environment.
	if len(envs) > 0 && len(envs[0]) > 0 {
		cmd.Env = append(os.Environ(), envs[0]...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		return nil, &CLIError{
			Command: strings.Join(args, " "),
			Stderr:  stderrStr,
		}
	}
	return stdout.Bytes(), nil
}

// getGitDir returns the .git directory path for a worktree.
func (c *CLI) getGitDir(worktreePath string) (string, error) {
	out, err := c.run(worktreePath, []string{"rev-parse", "--git-dir"})
	if err != nil {
		return "", fmt.Errorf("rev-parse --git-dir: %w", err)
	}
	gitDir := strings.TrimSpace(out)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreePath, gitDir)
	}
	return gitDir, nil
}

// pathspecExcludes returns exclude pathspec arguments for AlwaysSkipDirs.
func (c *CLI) pathspecExcludes() []string {
	var args []string
	for _, dir := range AlwaysSkipDirs {
		args = append(args, fmt.Sprintf(":(glob,exclude)**/%s/", dir))
	}
	return args
}

// fileExists checks if a file (or directory) exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// classifyPushError inspects a CLI error and returns a more specific error.
func classifyPushError(err error) error {
	if cliErr, ok := err.(*CLIError); ok {
		lower := strings.ToLower(cliErr.Stderr)
		if strings.Contains(lower, "non-fast-forward") ||
			strings.Contains(lower, "failed to push some refs") ||
			strings.Contains(lower, "fetch first") ||
			strings.Contains(lower, "updates were rejected") {
			return fmt.Errorf("push rejected (non-fast-forward): %s", cliErr.Stderr)
		}
		if strings.Contains(lower, "authentication failed") ||
			strings.Contains(lower, "could not read username") ||
			strings.Contains(lower, "invalid username or password") {
			return fmt.Errorf("push auth failed: %s", cliErr.Stderr)
		}
	}
	return fmt.Errorf("push: %w", err)
}

// classifyAuthError inspects a CLI error for authentication issues.
func classifyAuthError(err error) error {
	if cliErr, ok := err.(*CLIError); ok {
		lower := strings.ToLower(cliErr.Stderr)
		if strings.Contains(lower, "authentication failed") ||
			strings.Contains(lower, "could not read username") ||
			strings.Contains(lower, "invalid username or password") {
			return fmt.Errorf("auth failed: %s", cliErr.Stderr)
		}
	}
	return err
}
