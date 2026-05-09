package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Service provides Git operations combining go-git (reads) and CLI (writes).
type Service struct {
	cli *CLI
}

// NewService creates a new Git service.
func NewService() *Service {
	return &Service{cli: NewCLI()}
}

// --- Repository operations (go-git) ---

// OpenRepo opens a git repository at the given path.
func (s *Service) OpenRepo(repoPath string) (*git.Repository, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, &OpError{Op: "open_repo", Path: repoPath, Err: err}
	}
	return repo, nil
}

// IsRepoOpenable checks if a git repository can be opened at the given path.
func (s *Service) IsRepoOpenable(repoPath string) bool {
	_, err := git.PlainOpen(repoPath)
	return err == nil
}

// GetGitDir returns the .git directory path for a repository.
func (s *Service) GetGitDir(repoPath string) (string, error) {
	// go-git Repository doesn't expose Path() directly.
	// Use the filesystem path instead.
	dotGit := filepath.Join(repoPath, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", &OpError{Op: "get_git_dir", Path: repoPath, Err: err}
	}
	// If .git is a file (worktree), read the gitdir pointer.
	if !info.IsDir() {
		data, err := os.ReadFile(dotGit)
		if err != nil {
			return "", &OpError{Op: "get_git_dir", Path: repoPath, Err: err}
		}
		gitDir := strings.TrimSpace(string(data))
		gitDir = strings.TrimPrefix(gitDir, "gitdir: ")
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(repoPath, gitDir)
		}
		return gitDir, nil
	}
	return dotGit, nil
}

// GetCommonDir returns the common directory (for worktrees, the main repo).
func (s *Service) GetCommonDir(repoPath string) (string, error) {
	// For worktrees, the commondir is in the main repository.
	// Try reading the commondir file first.
	gitDir, err := s.GetGitDir(repoPath)
	if err != nil {
		return "", err
	}

	commonDirFile := filepath.Join(gitDir, "commondir")
	data, err := os.ReadFile(commonDirFile)
	if err == nil {
		commonDir := strings.TrimSpace(string(data))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(gitDir, commonDir)
		}
		return commonDir, nil
	}

	// Not a worktree, gitdir is the common dir.
	return gitDir, nil
}

// InitializeRepoWithMainBranch initializes a new git repository with a main branch.
func (s *Service) InitializeRepoWithMainBranch(repoPath string) error {
	// Use CLI to init with explicit branch name (go-git PlainInit always creates "master").
	cmd := exec.Command("git", "init", "-b", "main", repoPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return &OpError{Op: "init_repo", Path: repoPath, Err: fmt.Errorf("%s: %w", string(out), err)}
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return &OpError{Op: "init_open", Path: repoPath, Err: err}
	}

	// Set default identity for commits.
	cfg, err := repo.Config()
	if err != nil {
		return &OpError{Op: "init_config", Path: repoPath, Err: err}
	}
	cfg.User.Name = DefaultUserName
	cfg.User.Email = DefaultUserEmail
	if err := repo.SetConfig(cfg); err != nil {
		return &OpError{Op: "init_config", Path: repoPath, Err: err}
	}

	// Create an initial commit on the main branch.
	wt, err := repo.Worktree()
	if err != nil {
		return &OpError{Op: "init_worktree", Path: repoPath, Err: err}
	}

	// Write a placeholder .gitkeep to have something to commit.
	gitkeep := filepath.Join(repoPath, ".gitkeep")
	if err := os.WriteFile(gitkeep, []byte(""), 0o644); err != nil {
		return &OpError{Op: "init_gitkeep", Path: repoPath, Err: err}
	}

	if _, err := wt.Add(".gitkeep"); err != nil {
		return &OpError{Op: "init_add", Path: repoPath, Err: err}
	}

	if _, err := wt.Commit("Initial commit", &git.CommitOptions{
		Author:    s.defaultSignature(),
		Committer: s.defaultSignature(),
	}); err != nil {
		return &OpError{Op: "init_commit", Path: repoPath, Err: err}
	}

	return nil
}

// --- Branch operations (go-git reads) ---

// GetAllBranches returns all branches (local and remote) for a repository.
func (s *Service) GetAllBranches(repoPath string) ([]GitBranch, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return nil, err
	}

	headRef, _ := repo.Head()
	var currentBranch string
	if headRef != nil {
		currentBranch = headRef.Name().Short()
	}

	refs, err := repo.References()
	if err != nil {
		return nil, &OpError{Op: "get_branches", Path: repoPath, Err: err}
	}

	branchMap := make(map[string]*GitBranch)
	if err := refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name()
		if !name.IsBranch() && !name.IsRemote() {
			return nil
		}

		shortName := name.Short()
		// For remote branches, strip "origin/" prefix for display.
		displayName := shortName
		isRemote := name.IsRemote()

		if isRemote {
			// Skip HEAD references like "origin/HEAD".
			if strings.HasSuffix(shortName, "/HEAD") {
				return nil
			}
		}

		// Get last commit date.
		hash := ref.Hash()
		var lastDate time.Time
		if !hash.IsZero() {
			commit, err := repo.CommitObject(hash)
			if err == nil {
				lastDate = commit.Author.When
			}
		}

		branch := GitBranch{
			Name:           displayName,
			IsCurrent:      displayName == currentBranch,
			IsRemote:       isRemote,
			LastCommitDate: lastDate,
		}
		branchMap[displayName] = &branch
		return nil
	}); err != nil {
		return nil, &OpError{Op: "get_branches", Path: repoPath, Err: err}
	}

	branches := make([]GitBranch, 0, len(branchMap))
	for _, b := range branchMap {
		branches = append(branches, *b)
	}

	// Sort by name.
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

// GetBranchOID returns the commit SHA for a branch.
func (s *Service) GetBranchOID(repoPath, branchName string) (string, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return "", err
	}

	ref, err := findBranch(repo, branchName)
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// GetCurrentBranch returns the name of the current branch.
func (s *Service) GetCurrentBranch(repoPath string) (string, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", &OpError{Op: "get_current_branch", Path: repoPath, Err: ErrBranchNotFound}
	}
	return head.Name().Short(), nil
}

// CheckBranchExists checks if a branch exists.
func (s *Service) CheckBranchExists(repoPath, branchName string) (bool, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return false, err
	}

	_, err = findBranch(repo, branchName)
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// IsRemoteBranch checks if a branch is a remote branch.
func (s *Service) IsRemoteBranch(repoPath, branchName string) (bool, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return false, err
	}

	refName := plumbing.NewRemoteReferenceName("origin", branchName)
	_, err = repo.Reference(refName, true)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// IsBranchNameValid checks if a name is valid for a git branch.
func (s *Service) IsBranchNameValid(name string) bool {
	return isBranchNameValid(name)
}

// CreateBranch creates a new branch from a base branch.
func (s *Service) CreateBranch(repoPath, newBranch, baseBranch string) error {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return err
	}

	ref, err := findBranch(repo, baseBranch)
	if err != nil {
		return err
	}

	newRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(newBranch), ref.Hash())
	if err := repo.Storer.SetReference(newRef); err != nil {
		return &OpError{Op: "create_branch", Path: repoPath, Err: err}
	}
	return nil
}

// RenameLocalBranch renames a local branch.
func (s *Service) RenameLocalBranch(worktreePath, oldName, newName string) error {
	_, err := s.cli.run(worktreePath, []string{"branch", "-m", oldName, newName})
	return err
}

// DeleteBranch deletes a branch using CLI (handles worktree checks).
func (s *Service) DeleteBranch(repoPath, branchName string) error {
	return s.cli.DeleteBranch(repoPath, branchName)
}

// --- HEAD / commit operations (go-git) ---

// GetHeadCommit returns the HEAD commit of a repository.
func (s *Service) GetHeadCommit(repoPath string) *Commit {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return nil
	}

	head, err := repo.Head()
	if err != nil {
		return nil
	}

	return &Commit{Hash: head.Hash().String()}
}

// GetHeadInfo returns information about the current HEAD.
func (s *Service) GetHeadInfo(repoPath string) (*HeadInfo, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return nil, err
	}

	head, err := repo.Head()
	if err != nil {
		return nil, &OpError{Op: "get_head_info", Path: repoPath, Err: err}
	}

	info := &HeadInfo{
		OID: head.Hash().String(),
	}
	if head.Name().IsBranch() {
		info.Branch = head.Name().Short()
	}
	return info, nil
}

// IsHeadChildOf checks if HEAD is a descendant of the given commit.
func (s *Service) IsHeadChildOf(repoPath string, expectedParentOID string) bool {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return false
	}

	head, err := repo.Head()
	if err != nil {
		return false
	}

	parentHash := plumbing.NewHash(expectedParentOID)
	headHash := head.Hash()

	if headHash == parentHash {
		return true
	}

	// Walk ancestors of HEAD.
	commit, err := repo.CommitObject(headHash)
	if err != nil {
		return false
	}

	for _, parent := range commit.ParentHashes {
		if parent == parentHash {
			return true
		}
	}

	// Walk ancestors to check deeper.
	parentCommit, err := repo.CommitObject(parentHash)
	if err != nil {
		return false
	}
	return isAncestor(repo, parentCommit, commit)
}

// isAncestor checks if potentialAncestor is an ancestor of commit.
func isAncestor(repo *git.Repository, potentialAncestor, commit *object.Commit) bool {
	visited := make(map[plumbing.Hash]bool)
	queue := commit.ParentHashes
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if h == potentialAncestor.Hash {
			return true
		}
		if visited[h] {
			continue
		}
		visited[h] = true
		c, err := repo.CommitObject(h)
		if err != nil {
			continue
		}
		queue = append(queue, c.ParentHashes...)
	}
	return false
}

// GetBaseCommit returns the merge-base commit of two branches.
func (s *Service) GetBaseCommit(repoPath, branchName, baseBranchName string) (*Commit, error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return nil, err
	}

	branchRef, err := findBranch(repo, branchName)
	if err != nil {
		return nil, err
	}

	baseRef, err := findBranch(repo, baseBranchName)
	if err != nil {
		return nil, err
	}

	baseCommit, err := repo.CommitObject(baseRef.Hash())
	if err != nil {
		return nil, &OpError{Op: "get_base_commit", Path: repoPath, Err: err}
	}

	branchCommit, err := repo.CommitObject(branchRef.Hash())
	if err != nil {
		return nil, &OpError{Op: "get_base_commit", Path: repoPath, Err: err}
	}

	// Find merge base using ancestor traversal.
	mergeBase, err := findMergeBase(repo, baseCommit, branchCommit)
	if err != nil {
		return nil, &OpError{Op: "get_base_commit", Path: repoPath, Err: err}
	}
	if mergeBase == nil {
		return nil, &OpError{Op: "get_base_commit", Path: repoPath, Err: fmt.Errorf("no merge base found")}
	}

	return &Commit{Hash: mergeBase.Hash.String()}, nil
}

// GetBranchStatus returns ahead/behind counts for a branch relative to a base.
func (s *Service) GetBranchStatus(repoPath, branchName, baseBranchName string) (ahead, behind int, err error) {
	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return 0, 0, err
	}

	branchRef, err := findBranch(repo, branchName)
	if err != nil {
		return 0, 0, err
	}

	baseRef, err := findBranch(repo, baseBranchName)
	if err != nil {
		return 0, 0, err
	}

	return countAheadBehind(repo, baseRef.Hash(), branchRef.Hash())
}

// --- Worktree operations (CLI) ---

// AddWorktree creates a new worktree.
func (s *Service) AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error {
	return s.cli.WorktreeAdd(repoPath, worktreePath, branch, createBranch)
}

// RemoveWorktree removes a worktree.
func (s *Service) RemoveWorktree(repoPath, worktreePath string, force bool) error {
	return s.cli.WorktreeRemove(repoPath, worktreePath, force)
}

// MoveWorktree moves a worktree.
func (s *Service) MoveWorktree(repoPath, oldPath, newPath string) error {
	return s.cli.WorktreeMove(repoPath, oldPath, newPath)
}

// PruneWorktrees prunes stale worktree metadata.
func (s *Service) PruneWorktrees(repoPath string) error {
	return s.cli.WorktreePrune(repoPath)
}

// ValidateWorktree checks if a worktree is valid.
func (s *Service) ValidateWorktree(repoPath, worktreeName string) (bool, error) {
	entries, err := s.cli.ListWorktrees(repoPath)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if filepath.Base(entry.Path) == worktreeName {
			return true, nil
		}
	}
	return false, nil
}

// --- Worktree status (mixed) ---

// IsWorktreeClean checks if a worktree has no uncommitted changes (go-git).
func (s *Service) IsWorktreeClean(worktreePath string) (bool, error) {
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return false, &OpError{Op: "is_worktree_clean", Path: worktreePath, Err: err}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return false, &OpError{Op: "is_worktree_clean", Path: worktreePath, Err: err}
	}

	status, err := wt.Status()
	if err != nil {
		return false, &OpError{Op: "is_worktree_clean", Path: worktreePath, Err: err}
	}

	for _, s := range status {
		if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
			return false, nil
		}
	}
	return true, nil
}

// GetWorktreeStatus returns the full status of a worktree (CLI for untracked).
func (s *Service) GetWorktreeStatus(worktreePath string) (*WorktreeStatus, error) {
	return s.cli.GetWorktreeStatus(worktreePath)
}

// Commit stages all changes and commits with the given message (CLI).
func (s *Service) Commit(worktreePath, message string) (bool, error) {
	// Ensure identity is set.
	s.ensureCLICommitIdentity(worktreePath)

	hasChanges, err := s.cli.HasChanges(worktreePath)
	if err != nil {
		return false, err
	}
	if !hasChanges {
		return false, nil
	}

	if err := s.cli.AddAll(worktreePath); err != nil {
		return false, err
	}
	if err := s.cli.Commit(worktreePath, message); err != nil {
		return false, err
	}
	return true, nil
}

// ResetWorktreeToCommit resets a worktree to a specific commit (CLI).
func (s *Service) ResetWorktreeToCommit(worktreePath, commitSHA string, force bool) error {
	return s.cli.ResetHard(worktreePath, commitSHA)
}

// ReconcileWorktreeToCommit attempts to reset a worktree to a target commit,
// with safety checks for dirty state.
func (s *Service) ReconcileWorktreeToCommit(worktreePath, targetCommitOID string, opts WorktreeResetOptions) WorktreeResetOutcome {
	outcome := WorktreeResetOutcome{}

	head := s.GetHeadCommit(worktreePath)
	if head == nil || head.Hash == targetCommitOID {
		return outcome
	}
	outcome.Needed = true

	if !opts.PerformReset {
		return outcome
	}

	if opts.IsDirty && !opts.ForceWhenDirty {
		return outcome
	}

	if err := s.cli.ResetHard(worktreePath, targetCommitOID); err != nil {
		return outcome
	}
	outcome.Applied = true
	return outcome
}

// --- Merge (mixed) ---

// MergeChanges merges changes from a task branch into a base branch.
// Returns the new HEAD SHA on success.
func (s *Service) MergeChanges(baseWorktreePath, taskWorktreePath, taskBranchName, baseBranchName, commitMessage string) (string, error) {
	// Use CLI for squash merge since the base branch is checked out.
	return s.cli.MergeSquashCommit(baseWorktreePath, baseBranchName, taskBranchName, commitMessage)
}

// --- Rebase (CLI) ---

// RebaseBranch rebases a task branch onto a new base.
func (s *Service) RebaseBranch(repoPath, worktreePath, newBaseBranch, oldBaseBranch, taskBranch string) error {
	return s.cli.RebaseOnto(worktreePath, newBaseBranch, oldBaseBranch, taskBranch)
}

// AbortRebase aborts an in-progress rebase.
func (s *Service) AbortRebase(worktreePath string) error {
	return s.cli.AbortRebase(worktreePath)
}

// ContinueRebase continues an in-progress rebase.
func (s *Service) ContinueRebase(worktreePath string) error {
	return s.cli.ContinueRebase(worktreePath)
}

// --- Conflict detection (CLI) ---

// GetConflictedFiles returns files with merge conflicts.
func (s *Service) GetConflictedFiles(worktreePath string) ([]string, error) {
	return s.cli.GetConflictedFiles(worktreePath)
}

// DetectConflictOp determines what operation caused a conflict.
func (s *Service) DetectConflictOp(worktreePath string) (ConflictOp, bool, error) {
	return s.cli.DetectConflictOp(worktreePath)
}

// AbortConflicts aborts whatever conflict operation is in progress.
func (s *Service) AbortConflicts(worktreePath string) error {
	op, found, err := s.cli.DetectConflictOp(worktreePath)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	switch op {
	case ConflictOpRebase:
		return s.cli.AbortRebase(worktreePath)
	case ConflictOpMerge:
		return s.cli.AbortMerge(worktreePath)
	case ConflictOpCherryPick:
		return s.cli.AbortCherryPick(worktreePath)
	case ConflictOpRevert:
		return s.cli.AbortRevert(worktreePath)
	}
	return nil
}

// --- Remote operations (CLI) ---

// PushToRemote pushes a branch to the remote.
func (s *Service) PushToRemote(worktreePath, branchName string, force bool) error {
	remote, err := s.GetDefaultRemote(worktreePath)
	if err != nil {
		return err
	}
	return s.cli.Push(worktreePath, remote.URL, branchName, force)
}

// GetDefaultRemote returns the default remote for a repository.
func (s *Service) GetDefaultRemote(repoPath string) (*GitRemote, error) {
	remotes, err := s.cli.ListRemotes(repoPath)
	if err != nil {
		return nil, err
	}
	if len(remotes) == 0 {
		return nil, &OpError{Op: "get_default_remote", Path: repoPath, Err: fmt.Errorf("no remotes configured")}
	}

	// Prefer "origin".
	for _, r := range remotes {
		if r.Name == "origin" {
			return &r, nil
		}
	}
	// Fall back to first remote.
	return &remotes[0], nil
}

// GetRemoteBranchStatus returns ahead/behind counts for a remote-tracking branch.
func (s *Service) GetRemoteBranchStatus(repoPath, branchName string, baseBranchName *string) (ahead, behind int, err error) {
	remoteBranch := "origin/" + branchName
	localBase := "main"
	if baseBranchName != nil {
		localBase = *baseBranchName
	}
	remoteBase := "origin/" + localBase

	repo, err := s.OpenRepo(repoPath)
	if err != nil {
		return 0, 0, err
	}

	remoteBranchRef, err := findBranch(repo, remoteBranch)
	if err != nil {
		return 0, 0, err
	}

	remoteBaseRef, err := findBranch(repo, remoteBase)
	if err != nil {
		return 0, 0, err
	}

	return countAheadBehind(repo, remoteBaseRef.Hash(), remoteBranchRef.Hash())
}

// ListRemotes returns all remotes.
func (s *Service) ListRemotes(repoPath string) ([]GitRemote, error) {
	return s.cli.ListRemotes(repoPath)
}

// --- Diff (mixed) ---

// GetDiffs returns diffs between a worktree and a base commit.
func (s *Service) GetDiffs(worktreePath string, baseCommit *Commit, pathFilter []string, repoID string) ([]Diff, error) {
	// Use CLI diff status to get changed files.
	entries, err := s.cli.DiffStatus(worktreePath, baseCommit.Hash, pathFilter)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, nil
	}

	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return nil, &OpError{Op: "get_diffs", Path: worktreePath, Err: err}
	}

	baseHash := plumbing.NewHash(baseCommit.Hash)
	baseCommitObj, err := repo.CommitObject(baseHash)
	if err != nil {
		return nil, &OpError{Op: "get_diffs_base", Path: worktreePath, Err: err}
	}

	baseTree, err := baseCommitObj.Tree()
	if err != nil {
		return nil, &OpError{Op: "get_diffs_tree", Path: worktreePath, Err: err}
	}

	var diffs []Diff
	for _, entry := range entries {
		diff := Diff{
			RepoID: repoID,
		}

		switch entry.Change {
		case ChangeAdded:
			diff.Change = DiffChangeAdded
			diff.NewPath = entry.Path
		case ChangeModified:
			diff.Change = DiffChangeModified
			diff.OldPath = entry.Path
			diff.NewPath = entry.Path
		case ChangeDeleted:
			diff.Change = DiffChangeDeleted
			diff.OldPath = entry.Path
		case ChangeRenamed:
			diff.Change = DiffChangeRenamed
			diff.OldPath = entry.OldPath
			diff.NewPath = entry.Path
		default:
			continue
		}

		// Get old content from base tree.
		if diff.OldPath != "" {
			file, err := baseTree.File(diff.OldPath)
			if err == nil {
				content, err := file.Contents()
				if err == nil {
					if len(content) > MaxInlineDiffBytes {
						diff.ContentOmitted = true
					} else {
						diff.OldContent = content
					}
				}
			}
		}

		// Get new content from worktree.
		if diff.NewPath != "" && !diff.ContentOmitted {
			absPath := filepath.Join(worktreePath, diff.NewPath)
			data, err := os.ReadFile(absPath)
			if err == nil {
				if len(data) > MaxInlineDiffBytes {
					diff.ContentOmitted = true
				} else {
					diff.NewContent = string(data)
				}
			}
		}

		// Compute line change counts.
		diff.Additions, diff.Deletions = ComputeLineChangeCounts(diff.OldContent, diff.NewContent)

		diffs = append(diffs, diff)
	}

	return diffs, nil
}

// GetDiffFilePaths returns the set of file paths changed between worktree and base commit.
func (s *Service) GetDiffFilePaths(worktreePath string, baseCommit *Commit) (map[string]bool, error) {
	entries, err := s.cli.DiffStatus(worktreePath, baseCommit.Hash, nil)
	if err != nil {
		return nil, err
	}

	paths := make(map[string]bool)
	for _, entry := range entries {
		paths[entry.Path] = true
		if entry.OldPath != "" {
			paths[entry.OldPath] = true
		}
	}
	return paths, nil
}

// --- Private helpers ---

// findBranch looks up a branch by name, trying local first, then remote.
func findBranch(repo *git.Repository, name string) (*plumbing.Reference, error) {
	// Try local branch.
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(name), true)
	if err == nil {
		return ref, nil
	}

	// Try remote branch.
	ref, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", name), true)
	if err == nil {
		return ref, nil
	}

	return nil, &OpError{Op: "find_branch", Err: fmt.Errorf("%w: %s", ErrBranchNotFound, name)}
}

// countAheadBehind counts commits ahead and behind using BFS.
func countAheadBehind(repo *git.Repository, baseHash, branchHash plumbing.Hash) (ahead, behind int, err error) {
	if baseHash == branchHash {
		return 0, 0, nil
	}

	// Count ahead: commits reachable from branchHash but not baseHash.
	ahead, err = countReachable(repo, branchHash, baseHash)
	if err != nil {
		return 0, 0, err
	}

	// Count behind: commits reachable from baseHash but not branchHash.
	behind, err = countReachable(repo, baseHash, branchHash)
	if err != nil {
		return 0, 0, err
	}

	return ahead, behind, nil
}

// countReachable counts commits reachable from startHash but not from excludeHash.
func countReachable(repo *git.Repository, startHash, excludeHash plumbing.Hash) (int, error) {
	// Build set of ancestors of excludeHash.
	excludeSet := make(map[plumbing.Hash]bool)
	if !excludeHash.IsZero() {
		excludeCommit, err := repo.CommitObject(excludeHash)
		if err == nil {
			buildAncestorSet(repo, excludeCommit, excludeSet)
		}
	}

	// Walk from startHash, counting commits not in excludeSet.
	count := 0
	visited := make(map[plumbing.Hash]bool)
	queue := []plumbing.Hash{startHash}

	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]

		if visited[h] {
			continue
		}
		visited[h] = true

		if excludeSet[h] {
			continue
		}

		count++

		commit, err := repo.CommitObject(h)
		if err != nil {
			break
		}
		for _, parent := range commit.ParentHashes {
			if !visited[parent] {
				queue = append(queue, parent)
			}
		}
	}

	return count, nil
}

// buildAncestorSet builds a set of all ancestors of a commit.
func buildAncestorSet(repo *git.Repository, commit *object.Commit, set map[plumbing.Hash]bool) {
	queue := []*object.Commit{commit}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]

		if set[c.Hash] {
			continue
		}
		set[c.Hash] = true

		for _, parentHash := range c.ParentHashes {
			if !set[parentHash] {
				parent, err := repo.CommitObject(parentHash)
				if err == nil {
					queue = append(queue, parent)
				}
			}
		}
	}
}

// findMergeBase finds the merge-base of two commits.
func findMergeBase(repo *git.Repository, a, b *object.Commit) (*object.Commit, error) {
	// Collect all ancestors of a.
	ancestorsA := make(map[plumbing.Hash]bool)
	queue := []*object.Commit{a}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if ancestorsA[c.Hash] {
			continue
		}
		ancestorsA[c.Hash] = true
		for _, ph := range c.ParentHashes {
			p, err := repo.CommitObject(ph)
			if err == nil && !ancestorsA[p.Hash] {
				queue = append(queue, p)
			}
		}
	}

	// BFS from b, first ancestor also in A is the merge base.
	visited := make(map[plumbing.Hash]bool)
	queue = []*object.Commit{b}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if visited[c.Hash] {
			continue
		}
		visited[c.Hash] = true

		if ancestorsA[c.Hash] {
			return c, nil
		}

		for _, ph := range c.ParentHashes {
			p, err := repo.CommitObject(ph)
			if err == nil && !visited[p.Hash] {
				queue = append(queue, p)
			}
		}
	}

	return nil, nil
}

// ensureCLICommitIdentity ensures user.name and user.email are set in the repo config.
func (s *Service) ensureCLICommitIdentity(worktreePath string) {
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return
	}
	cfg, err := repo.Config()
	if err != nil {
		return
	}
	changed := false
	if cfg.User.Name == "" {
		cfg.User.Name = DefaultUserName
		changed = true
	}
	if cfg.User.Email == "" {
		cfg.User.Email = DefaultUserEmail
		changed = true
	}
	if changed {
		_ = repo.SetConfig(cfg)
	}
}

// defaultSignature returns a git signature with the default identity.
func (s *Service) defaultSignature() *object.Signature {
	return &object.Signature{
		Name:  DefaultUserName,
		Email: DefaultUserEmail,
		When:  time.Now(),
	}
}
