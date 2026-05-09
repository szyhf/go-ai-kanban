package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/git"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
)

// RepoService provides business logic for git repository management.
type RepoService struct {
	repoRepo *repository.GitRepoRepo
	gitSvc   *git.Service
}

// NewRepoService creates a new RepoService.
func NewRepoService(repoRepo *repository.GitRepoRepo, gitSvc *git.Service) *RepoService {
	return &RepoService{repoRepo: repoRepo, gitSvc: gitSvc}
}

// NormalizePath expands ~ and returns an absolute path.
func (s *RepoService) NormalizePath(path string) (string, error) {
	expanded, err := expandTilde(path)
	if err != nil {
		return "", fmt.Errorf("normalize path: %w", err)
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("normalize path: %w", err)
	}
	return abs, nil
}

// Register registers a git repository by path.
// If display_name is empty, the directory name is used.
func (s *RepoService) Register(path, displayName string) (*domain.Repo, error) {
	normalized, err := s.NormalizePath(path)
	if err != nil {
		return nil, err
	}

	if err := s.validateGitRepoPath(normalized); err != nil {
		return nil, err
	}

	name := filepath.Base(normalized)
	dn := displayName
	if dn == "" {
		dn = name
	}

	return s.repoRepo.FindOrCreate(normalized, name, dn)
}

// FindByID returns a repo by ID. Returns nil, nil if not found.
func (s *RepoService) FindByID(id domain.UUID) (*domain.Repo, error) {
	return s.repoRepo.FindByID(id)
}

// GetByID returns a repo by ID. Returns an error if not found.
func (s *RepoService) GetByID(id domain.UUID) (*domain.Repo, error) {
	repo, err := s.repoRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("repo not found: %s", id)
	}
	return repo, nil
}

// FindAll returns all registered repos.
func (s *RepoService) FindAll() ([]domain.Repo, error) {
	return s.repoRepo.FindAll()
}

// Update updates a repo.
func (s *RepoService) Update(id domain.UUID, update domain.UpdateRepo) error {
	return s.repoRepo.Update(id, update)
}

// Delete deletes a repo by ID.
func (s *RepoService) Delete(id domain.UUID) error {
	return s.repoRepo.Delete(id)
}

// InitRepo creates a new git repository and registers it.
func (s *RepoService) InitRepo(parentPath, folderName string) (*domain.Repo, error) {
	// Validate folder name.
	folderName = strings.TrimSpace(folderName)
	if folderName == "" {
		return nil, fmt.Errorf("folder name cannot be empty")
	}
	if strings.Contains(folderName, "/") || strings.Contains(folderName, string(filepath.Separator)) {
		return nil, fmt.Errorf("folder name cannot contain slashes: %q", folderName)
	}
	if folderName == "." || folderName == ".." {
		return nil, fmt.Errorf("invalid folder name: %q", folderName)
	}

	normalizedParent, err := s.NormalizePath(parentPath)
	if err != nil {
		return nil, err
	}

	// Validate parent exists and is a directory.
	info, err := os.Stat(normalizedParent)
	if err != nil {
		return nil, fmt.Errorf("parent path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("parent path is not a directory: %s", normalizedParent)
	}

	repoPath := filepath.Join(normalizedParent, folderName)

	// Check target does not already exist.
	if _, err := os.Stat(repoPath); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", repoPath)
	}

	// Initialize the git repo.
	if err := s.gitSvc.InitializeRepoWithMainBranch(repoPath); err != nil {
		return nil, fmt.Errorf("init repo: %w", err)
	}

	return s.Register(repoPath, folderName)
}

// SearchFiles searches for files across multiple repos using a simple substring match.
func (s *RepoService) SearchFiles(repos []domain.Repo, query string) ([]domain.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(repos) == 0 {
		return nil, nil
	}

	var results []domain.SearchResult
	for _, repo := range repos {
		repoResults := searchRepoFiles(repo.Path, repo.Name, query)
		results = append(results, repoResults...)
	}

	// Sort by match type priority then by score descending.
	sort.Slice(results, func(i, j int) bool {
		ri, rj := results[i], results[j]
		if ri.MatchType != rj.MatchType {
			return matchTypePriority(ri.MatchType) < matchTypePriority(rj.MatchType)
		}
		return ri.Score > rj.Score
	})

	// Truncate to 10 results.
	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}

// validateGitRepoPath checks that the path exists, is a directory, and is a git repo.
func (s *RepoService) validateGitRepoPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path not found: %s", path)
		}
		return fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	if !s.gitSvc.IsRepoOpenable(path) {
		return fmt.Errorf("not a git repository: %s", path)
	}
	return nil
}

// expandTilde replaces ~ with the home directory.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}

// searchRepoFiles walks a repo and finds files matching the query.
func searchRepoFiles(repoPath, repoName, query string) []domain.SearchResult {
	lowerQuery := strings.ToLower(query)
	var results []domain.SearchResult

	filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}

		// Skip hidden and common non-project directories.
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && containsString(git.AlwaysSkipDirs, name) {
			return filepath.SkipDir
		}

		relPath, _ := filepath.Rel(repoPath, path)
		displayPath := filepath.Join(repoName, relPath)
		lowerName := strings.ToLower(name)
		lowerRelPath := strings.ToLower(relPath)

		var matchType domain.SearchMatchType
		var score int

		if lowerName == lowerQuery {
			matchType = domain.SearchMatchFileName
			score = 100
		} else if strings.Contains(lowerName, lowerQuery) {
			matchType = domain.SearchMatchFileName
			score = 50
		} else if strings.Contains(lowerRelPath, lowerQuery) {
			matchType = domain.SearchMatchFullPath
			score = 25
		} else {
			return nil
		}

		results = append(results, domain.SearchResult{
			Path:      displayPath,
			MatchType: matchType,
			Score:     int64(score),
		})
		return nil
	})

	return results
}

// matchTypePriority returns sort priority for a match type (lower = better).
func matchTypePriority(mt domain.SearchMatchType) int {
	switch mt {
	case domain.SearchMatchFileName:
		return 0
	case domain.SearchMatchDirectoryName:
		return 1
	case domain.SearchMatchFullPath:
		return 2
	default:
		return 3
	}
}

// containsString checks if a string is in a slice.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
