package githost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GhCLI wraps the GitHub CLI (`gh`) for PR operations.
type GhCLI struct{}

// NewGhCLI creates a new GhCLI instance.
func NewGhCLI() *GhCLI { return &GhCLI{} }

// IsInstalled checks if the `gh` CLI is available in PATH.
func (g *GhCLI) IsInstalled() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// PRDetail represents a pull request.
type PRDetail struct {
	Number        int     `json:"number"`
	URL           string  `json:"url"`
	Status        string  `json:"status"` // "open", "merged", "closed", "unknown"
	MergedAt      *string `json:"merged_at"`
	MergeCommitSHA *string `json:"merge_commit_sha"`
	Title         string  `json:"title"`
	BaseBranch    string  `json:"base_branch"`
	HeadBranch    string  `json:"head_branch"`
}

// PRComment represents a PR comment.
type PRComment struct {
	CommentType        string  `json:"comment_type"` // "general" or "review"
	ID                 string  `json:"id"`
	Author             string  `json:"author"`
	AuthorAssociation  *string `json:"author_association"`
	Body               string  `json:"body"`
	CreatedAt          string  `json:"created_at"`
	URL                *string `json:"url"`
	// Review comment fields
	Path     *string `json:"path,omitempty"`
	Line     *int    `json:"line,omitempty"`
	Side     *string `json:"side,omitempty"`
	DiffHunk *string `json:"diff_hunk,omitempty"`
}

// RepoInfo contains basic repo metadata from `gh repo view`.
type RepoInfo struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

// CreatePROptions are the options for creating a PR.
type CreatePROptions struct {
	Repo     string // owner/repo
	Head     string // head branch
	Base     string // base branch
	Title    string
	Body     string
	Draft    bool
}

// CreatePR creates a pull request and returns the PR URL.
func (g *GhCLI) CreatePR(opts CreatePROptions) (string, error) {
	args := []string{
		"pr", "create",
		"--repo", opts.Repo,
		"--head", opts.Head,
		"--base", opts.Base,
		"--title", opts.Title,
	}
	if opts.Body != "" {
		// Write body to temp file to avoid shell escaping issues.
		tmpFile, err := os.CreateTemp("", "pr-body-*.md")
		if err != nil {
			return "", fmt.Errorf("create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.WriteString(opts.Body); err != nil {
			return "", fmt.Errorf("write temp file: %w", err)
		}
		tmpFile.Close()
		args = append(args, "--body-file", tmpFile.Name())
	}
	if opts.Draft {
		args = append(args, "--draft")
	}

	out, err := g.run(args...)
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// ListPRsForBranch lists PRs for a specific branch.
func (g *GhCLI) ListPRsForBranch(repo, branch string) ([]PRDetail, error) {
	out, err := g.run(
		"pr", "list",
		"--repo", repo,
		"--state", "all",
		"--head", branch,
		"--json", "number,url,state,mergedAt,mergeCommit,title,baseRefName,headRefName",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}

	var raw []struct {
		Number       int     `json:"number"`
		URL          string  `json:"url"`
		State        string  `json:"state"`
		MergedAt     *string `json:"mergedAt"`
		MergeCommit  *string `json:"mergeCommit"`
		Title        string  `json:"title"`
		BaseRefName  string  `json:"baseRefName"`
		HeadRefName  string  `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse pr list: %w", err)
	}

	var prs []PRDetail
	for _, r := range raw {
		status := r.State
		if r.MergedAt != nil && *r.MergedAt != "" {
			status = "merged"
		}
		prs = append(prs, PRDetail{
			Number:         r.Number,
			URL:            r.URL,
			Status:         status,
			MergedAt:       r.MergedAt,
			MergeCommitSHA: r.MergeCommit,
			Title:          r.Title,
			BaseBranch:     r.BaseRefName,
			HeadBranch:     r.HeadRefName,
		})
	}
	return prs, nil
}

// ListOpenPRs lists open and recently closed PRs for a repo.
func (g *GhCLI) ListOpenPRs(repo string) ([]PRDetail, error) {
	// Get open PRs.
	out, err := g.run(
		"pr", "list",
		"--repo", repo,
		"--state", "open",
		"--json", "number,url,state,title,baseRefName,headRefName,updatedAt",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr list open: %w", err)
	}

	var raw []struct {
		Number      int    `json:"number"`
		URL         string `json:"url"`
		State       string `json:"state"`
		Title       string `json:"title"`
		BaseRefName string `json:"baseRefName"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse pr list: %w", err)
	}

	prs := make([]PRDetail, 0, len(raw))
	for _, r := range raw {
		prs = append(prs, PRDetail{
			Number:     r.Number,
			URL:        r.URL,
			Status:     "open",
			Title:      r.Title,
			BaseBranch: r.BaseRefName,
			HeadBranch: r.HeadRefName,
		})
	}
	return prs, nil
}

// GetPRInfo fetches details for a single PR by URL or number.
func (g *GhCLI) GetPRInfo(repo string, prRef string) (*PRDetail, error) {
	args := []string{
		"pr", "view", prRef,
		"--repo", repo,
		"--json", "number,url,state,mergedAt,mergeCommit,title,baseRefName,headRefName",
	}
	out, err := g.run(args...)
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %w", err)
	}

	var raw struct {
		Number      int     `json:"number"`
		URL         string  `json:"url"`
		State       string  `json:"state"`
		MergedAt    *string `json:"mergedAt"`
		MergeCommit *string `json:"mergeCommit"`
		Title       string  `json:"title"`
		BaseRefName string  `json:"baseRefName"`
		HeadRefName string  `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse pr view: %w", err)
	}

	status := raw.State
	if raw.MergedAt != nil && *raw.MergedAt != "" {
		status = "merged"
	}
	return &PRDetail{
		Number:         raw.Number,
		URL:            raw.URL,
		Status:         status,
		MergedAt:       raw.MergedAt,
		MergeCommitSHA: raw.MergeCommit,
		Title:          raw.Title,
		BaseBranch:     raw.BaseRefName,
		HeadBranch:     raw.HeadRefName,
	}, nil
}

// GetPRComments fetches both general and review comments for a PR.
func (g *GhCLI) GetPRComments(repo string, prNumber int) ([]PRComment, error) {
	// Get general comments via pr view --json comments.
	out, err := g.run(
		"pr", "view", fmt.Sprintf("%d", prNumber),
		"--repo", repo,
		"--json", "comments",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr view comments: %w", err)
	}

	var generalResp struct {
		Comments []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			AuthorAssociation string `json:"authorAssociation"`
			Body              string `json:"body"`
			CreatedAt         string `json:"createdAt"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &generalResp); err != nil {
		return nil, fmt.Errorf("parse pr comments: %w", err)
	}

	var comments []PRComment
	for _, c := range generalResp.Comments {
		assoc := c.AuthorAssociation
		comments = append(comments, PRComment{
			CommentType:       "general",
			Author:            c.Author.Login,
			AuthorAssociation: &assoc,
			Body:              c.Body,
			CreatedAt:         c.CreatedAt,
		})
	}

	// Get review comments via API.
	apiOut, err := g.run(
		"api",
		fmt.Sprintf("repos/%s/pulls/%d/comments", repo, prNumber),
	)
	if err != nil {
		// Review comments may not exist; non-fatal.
		return comments, nil
	}

	var reviewComments []struct {
		ID     int    `json:"id"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		Path      string `json:"path"`
		Line      *int   `json:"line"`
		Side      string `json:"side"`
		DiffHunk  string `json:"diff_hunk"`
	}
	if err := json.Unmarshal([]byte(apiOut), &reviewComments); err != nil {
		return comments, nil
	}

	for _, rc := range reviewComments {
		cmt := PRComment{
			CommentType: "review",
			ID:          fmt.Sprintf("%d", rc.ID),
			Author:      rc.User.Login,
			Body:        rc.Body,
			CreatedAt:   rc.CreatedAt,
			Path:        &rc.Path,
			DiffHunk:    &rc.DiffHunk,
		}
		if rc.Line != nil {
			cmt.Line = rc.Line
		}
		if rc.Side != "" {
			cmt.Side = &rc.Side
		}
		comments = append(comments, cmt)
	}

	return comments, nil
}

// PRCheckout checks out a PR branch.
func (g *GhCLI) PRCheckout(repo string, prNumber int) error {
	_, err := g.run(
		"pr", "checkout", fmt.Sprintf("%d", prNumber),
		"--repo", repo,
		"--force",
	)
	if err != nil {
		return fmt.Errorf("gh pr checkout: %w", err)
	}
	return nil
}

// GetRepoInfo fetches repo metadata from a URL.
func (g *GhCLI) GetRepoInfo(repoURL string) (*RepoInfo, error) {
	out, err := g.run(
		"repo", "view", repoURL,
		"--json", "owner,name,url",
	)
	if err != nil {
		return nil, fmt.Errorf("gh repo view: %w", err)
	}

	var raw struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse repo info: %w", err)
	}
	return &RepoInfo{
		Owner: raw.Owner.Login,
		Name:  raw.Name,
		URL:   raw.URL,
	}, nil
}

// ParseRepoFromURL extracts owner/repo from a GitHub URL.
func ParseRepoFromURL(rawURL string) (string, error) {
	// Handle various GitHub URL formats:
	// https://github.com/owner/repo
	// git@github.com:owner/repo.git
	// owner/repo
	s := strings.TrimSpace(rawURL)
	s = strings.TrimSuffix(s, ".git")

	if !strings.Contains(s, "/") {
		return "", fmt.Errorf("invalid repo URL: %s", rawURL)
	}

	// SSH format: git@github.com:owner/repo
	if strings.HasPrefix(s, "git@") {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
	}

	// HTTPS format: https://github.com/owner/repo
	if strings.Contains(s, "://") {
		parts := strings.SplitN(s, "://", 2)
		if len(parts) == 2 {
			pathParts := strings.SplitN(parts[1], "/", 3)
			if len(pathParts) >= 3 {
				return pathParts[1] + "/" + pathParts[2], nil
			}
		}
	}

	// Already owner/repo format.
	if !strings.Contains(s, " ") && strings.Count(s, "/") == 1 {
		return s, nil
	}

	return "", fmt.Errorf("cannot parse repo from URL: %s", rawURL)
}

// run executes a gh command and returns stdout.
func (g *GhCLI) run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GH_NO_UPDATE_NOTIFIER=1")

	if err := cmd.Run(); err != nil {
		return "", &GhError{
			Command: strings.Join(args, " "),
			Stderr:  stderr.String(),
		}
	}
	return stdout.String(), nil
}

// GhError represents a gh CLI failure.
type GhError struct {
	Command string
	Stderr  string
}

func (e *GhError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("gh %s: %s", e.Command, e.Stderr)
	}
	return fmt.Sprintf("gh %s failed", e.Command)
}
