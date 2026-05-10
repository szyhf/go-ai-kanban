package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/git"
)

// DirectoryEntry represents a file or directory entry in a listing.
type DirectoryEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDirectory  bool   `json:"is_directory"`
	IsGitRepo    bool   `json:"is_git_repo"`
	LastModified *int64 `json:"last_modified"`
}

// DirectoryListResponse is the response for listing a directory.
type DirectoryListResponse struct {
	Entries     []DirectoryEntry `json:"entries"`
	CurrentPath string           `json:"current_path"`
}

// FilesystemService provides filesystem operations for discovering git repos.
type FilesystemService struct {
	gitSvc *git.Service
}

// NewFilesystemService creates a new FilesystemService.
func NewFilesystemService(gitSvc *git.Service) *FilesystemService {
	return &FilesystemService{gitSvc: gitSvc}
}

// ListGitRepos recursively finds git repositories under the given path.
// If path is empty, the home directory is used.
func (s *FilesystemService) ListGitRepos(path string, maxDepth int) ([]DirectoryEntry, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = home
	}

	if err := verifyDirectory(path); err != nil {
		return nil, err
	}

	return s.listGitReposInner([]string{path}, maxDepth)
}

// ListCommonGitRepos searches common directories for git repositories.
func (s *FilesystemService) ListCommonGitRepos(maxDepth int) ([]DirectoryEntry, error) {
	var paths []string

	// Current working directory.
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, cwd)
	}

	// Home directory.
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	paths = append(paths, home)

	// Common project subdirectories.
	for _, sub := range []string{"repos", "dev", "work", "code", "projects"} {
		p := filepath.Join(home, sub)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			paths = append(paths, p)
		}
	}

	return s.listGitReposInner(paths, maxDepth)
}

// ListDirectory returns a single-level directory listing.
func (s *FilesystemService) ListDirectory(path string) (*DirectoryListResponse, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = home
	}

	if err := verifyDirectory(path); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var result []DirectoryEntry
	for _, e := range entries {
		name := e.Name()
		// Skip hidden files/directories.
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(path, name)
		isDir := info.IsDir()
		isGitRepo := false
		if isDir {
			isGitRepo = s.gitSvc.IsRepoOpenable(fullPath)
		}

		result = append(result, DirectoryEntry{
			Name:        name,
			Path:        fullPath,
			IsDirectory: isDir,
			IsGitRepo:   isGitRepo,
		})
	}

	// Sort: directories first, then files; within each group, alphabetical.
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDirectory != result[j].IsDirectory {
			return result[i].IsDirectory
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return &DirectoryListResponse{
		Entries:     result,
		CurrentPath: path,
	}, nil
}

// listGitReposInner recursively walks the given paths to find git repos.
func (s *FilesystemService) listGitReposInner(paths []string, maxDepth int) ([]DirectoryEntry, error) {
	skipDirs := getDirectoriesToSkip()
	seen := make(map[string]bool)
	var results []DirectoryEntry

	for _, base := range paths {
		filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}

			// Calculate depth relative to base.
			rel, _ := filepath.Rel(base, path)
			depth := 0
			if rel != "." {
				depth = len(strings.Split(rel, string(filepath.Separator)))
			}

			name := d.Name()

			// Skip if beyond max depth.
			if maxDepth > 0 && depth > maxDepth {
				return nil
			}
			// At max depth boundary: check for git repo but don't recurse further.
			if maxDepth > 0 && depth == maxDepth && d.IsDir() {
				gitPath := filepath.Join(path, ".git")
				if _, err := os.Stat(gitPath); err == nil {
					if !seen[path] {
						seen[path] = true
						results = append(results, DirectoryEntry{
							Name:        name,
							Path:        path,
							IsDirectory: true,
							IsGitRepo:   true,
						})
					}
				}
				return filepath.SkipDir
			}

			// Skip hidden directories.
			if strings.HasPrefix(name, ".") && name != "." {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			// Skip known non-project directories.
			if skipDirs[name] {
				return filepath.SkipDir
			}

			// Check for git repo.
			gitPath := filepath.Join(path, ".git")
			if _, err := os.Stat(gitPath); err == nil {
				if !seen[path] {
					seen[path] = true
					var modTime *int64
					if info, err := d.Info(); err == nil {
						if mtime := info.ModTime().Unix(); mtime != 0 {
							elapsed := time.Since(info.ModTime()).Seconds()
							secs := int64(elapsed)
							modTime = &secs
						}
					}
					results = append(results, DirectoryEntry{
						Name:         name,
						Path:         path,
						IsDirectory:  true,
						IsGitRepo:    true,
						LastModified: modTime,
					})
				}
				// Don't recurse into git repos.
				return filepath.SkipDir
			}

			return nil
		})
	}

	// Sort by last_modified ascending.
	sort.Slice(results, func(i, j int) bool {
		mi, mj := results[i].LastModified, results[j].LastModified
		if mi == nil && mj == nil {
			return false
		}
		if mi == nil {
			return true
		}
		if mj == nil {
			return false
		}
		return *mi < *mj
	})

	return results, nil
}

// getDirectoriesToSkip returns a set of directory names to skip during repo scanning.
func getDirectoriesToSkip() map[string]bool {
	skip := map[string]bool{
		"node_modules": true,
		"target":       true,
		"build":        true,
		"dist":         true,
		".next":        true,
		".nuxt":        true,
		".cache":       true,
		".npm":         true,
		".yarn":        true,
		".pnpm-store":  true,
		"Library":      true,
		"AppData":      true,
		"Applications": true,
	}

	// Add known system directories.
	for _, p := range []string{
		homeDir("Downloads"),
		homeDir("Pictures"),
		homeDir("Videos"),
		homeDir("Music"),
		homeDir("Documents"),
	} {
		if p != "" {
			skip[filepath.Base(p)] = true
		}
	}

	return skip
}

// homeDir returns a subdirectory of the home directory.
func homeDir(sub string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, sub)
}

// verifyDirectory checks that the path exists and is a directory.
func verifyDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}
