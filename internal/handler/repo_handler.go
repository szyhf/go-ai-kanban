package handler

import (
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/githost"
)

// registerRepoRoutes registers repo-related routes.
func (h *Handler) registerRepoRoutes(r chi.Router) {
	r.Get("/", h.listRepos)
	r.Post("/", h.registerRepo)
	r.Post("/init", h.initRepo)
	r.Get("/pr-info", h.getPRInfo)
	r.Get("/recent", h.listRecentRepos)
	r.Post("/batch", h.batchGetReposByIDs)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getRepo)
		r.Put("/", h.updateRepo)
		r.Delete("/", h.deleteRepo)
		r.Get("/branches", h.listBranches)
		r.Get("/prs", h.listRepoPRs)
		r.Get("/remotes", h.listRepoRemotes)
		r.Get("/search", h.searchRepoFiles)
		r.Post("/open-editor", h.openRepoEditor)
	})
}

// listRepos handles GET /api/repos.
func (h *Handler) listRepos(w http.ResponseWriter, r *http.Request) {
	// Check for batch get via ids[] query param.
	if ids := getQuerySlice(r, "ids[]"); len(ids) > 0 {
		h.batchGetRepos(w, r, ids)
		return
	}

	repos, err := h.repoSvc.FindAll()
	if err != nil {
		internalError(w, "failed to list repos: "+err.Error())
		return
	}
	success(w, repos)
}

// batchGetRepos handles batch retrieval by IDs.
func (h *Handler) batchGetRepos(w http.ResponseWriter, _ *http.Request, idStrs []string) {
	var repos []domain.Repo
	for _, s := range idStrs {
		id, err := domain.ParseUUID(s)
		if err != nil {
			continue
		}
		r, err := h.repoSvc.FindByID(id)
		if err != nil || r == nil {
			continue
		}
		repos = append(repos, *r)
	}
	success(w, repos)
}

// registerRepo handles POST /api/repos.
func (h *Handler) registerRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path        string `json:"path"`
		DisplayName string `json:"display_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Path == "" {
		badRequest(w, "path is required")
		return
	}

	repo, err := h.repoSvc.Register(req.Path, req.DisplayName)
	if err != nil {
		internalError(w, "failed to register repo: "+err.Error())
		return
	}
	created(w, repo)
}

// initRepo handles POST /api/repos/init.
func (h *Handler) initRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentPath string `json:"parent_path"`
		FolderName string `json:"folder_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ParentPath == "" || req.FolderName == "" {
		badRequest(w, "parent_path and folder_name are required")
		return
	}

	repo, err := h.repoSvc.InitRepo(req.ParentPath, req.FolderName)
	if err != nil {
		internalError(w, "failed to init repo: "+err.Error())
		return
	}
	created(w, repo)
}

// getRepo handles GET /api/repos/{id}.
func (h *Handler) getRepo(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil {
		notFound(w, "repo not found")
		return
	}
	success(w, repo)
}

// updateRepo handles PUT /api/repos/{id}.
func (h *Handler) updateRepo(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var update domain.UpdateRepo
	if !decodeJSON(w, r, &update) {
		return
	}

	if err := h.repoSvc.Update(id, update); err != nil {
		internalError(w, "failed to update repo: "+err.Error())
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil {
		success(w, map[string]string{"status": "updated"})
		return
	}
	success(w, repo)
}

// deleteRepo handles DELETE /api/repos/{id}.
func (h *Handler) deleteRepo(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.repoSvc.Delete(id); err != nil {
		internalError(w, "failed to delete repo: "+err.Error())
		return
	}
	success(w, map[string]string{"status": "deleted"})
}

// listBranches handles GET /api/repos/{id}/branches.
func (h *Handler) listBranches(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil {
		notFound(w, "repo not found")
		return
	}

	branches, err := h.gitSvc.GetAllBranches(repo.Path)
	if err != nil {
		internalError(w, "failed to list branches: "+err.Error())
		return
	}
	success(w, branches)
}

// --- PR and remote endpoints ---

// listRepoPRs handles GET /api/repos/{id}/prs.
func (h *Handler) listRepoPRs(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil || repo == nil {
		notFound(w, "repo not found")
		return
	}

	if !h.ghCLI.IsInstalled() {
		errorWithData(w, http.StatusBadRequest, "GitHub CLI not installed", map[string]string{
			"type":     "cli_not_installed",
			"provider": "GitHub",
		})
		return
	}

	remoteName := getQuery(r, "remote")
	if remoteName == "" {
		remoteName = "origin"
	}

	remote, err := h.gitSvc.GetDefaultRemote(repo.Path)
	if err != nil {
		internalError(w, "failed to get remote: "+err.Error())
		return
	}
	repoSlug, err := githost.ParseRepoFromURL(remote.URL)
	if err != nil {
		internalError(w, "failed to parse repo URL: "+err.Error())
		return
	}

	prs, err := h.ghCLI.ListOpenPRs(repoSlug)
	if err != nil {
		slog.Warn("列出 PR 失败", "error", err)
		errorWithData(w, http.StatusInternalServerError, "failed to list PRs", map[string]string{
			"type": "cli_error",
		})
		return
	}
	success(w, prs)
}

// listRepoRemotes handles GET /api/repos/{id}/remotes.
func (h *Handler) listRepoRemotes(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil || repo == nil {
		notFound(w, "repo not found")
		return
	}

	remotes, err := h.gitSvc.ListRemotes(repo.Path)
	if err != nil {
		internalError(w, "failed to list remotes: "+err.Error())
		return
	}
	success(w, remotes)
}

// getPRInfo handles GET /api/repos/pr-info?url=...
func (h *Handler) getPRInfo(w http.ResponseWriter, r *http.Request) {
	prURL := getQuery(r, "url")
	if prURL == "" {
		badRequest(w, "url query parameter is required")
		return
	}

	if !h.ghCLI.IsInstalled() {
		errorWithData(w, http.StatusBadRequest, "GitHub CLI not installed", map[string]string{
			"type":     "cli_not_installed",
			"provider": "GitHub",
		})
		return
	}

	repoInfo, err := h.ghCLI.GetRepoInfo(prURL)
	if err != nil {
		internalError(w, "failed to get repo info: "+err.Error())
		return
	}

	pr, err := h.ghCLI.GetPRInfo(repoInfo.Owner+"/"+repoInfo.Name, prURL)
	if err != nil {
		internalError(w, "failed to get PR info: "+err.Error())
		return
	}
	success(w, pr)
}

// listRecentRepos handles GET /api/repos/recent.
func (h *Handler) listRecentRepos(w http.ResponseWriter, r *http.Request) {
	// Return all repos sorted by most recently updated.
	repos, err := h.repoSvc.FindAll()
	if err != nil {
		internalError(w, "failed to list repos: "+err.Error())
		return
	}
	success(w, repos)
}

// batchGetReposByIDs handles POST /api/repos/batch.
func (h *Handler) batchGetReposByIDs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	var repos []domain.Repo
	for _, s := range req.IDs {
		id, err := domain.ParseUUID(s)
		if err != nil {
			continue
		}
		r, err := h.repoSvc.GetByID(id)
		if err != nil || r == nil {
			continue
		}
		repos = append(repos, *r)
	}
	success(w, repos)
}

// searchRepoFiles handles GET /api/repos/{id}/search?q=...&mode=...
func (h *Handler) searchRepoFiles(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	query := getQuery(r, "q")
	if query == "" {
		badRequest(w, "q is required")
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil || repo == nil {
		notFound(w, "repo not found")
		return
	}

	results, err := h.repoSvc.SearchFiles([]domain.Repo{*repo}, query)
	if err != nil {
		internalError(w, "search failed: "+err.Error())
		return
	}
	success(w, results)
}

// openRepoEditor handles POST /api/repos/{id}/open-editor.
// Opens the repo in the user's configured editor.
func (h *Handler) openRepoEditor(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	repo, err := h.repoSvc.GetByID(id)
	if err != nil || repo == nil {
		notFound(w, "repo not found")
		return
	}

	var req struct {
		EditorType string `json:"editor_type"`
	}
	decodeJSON(w, r, &req)

	command := "code"
	switch strings.ToUpper(req.EditorType) {
	case "CURSOR":
		command = "cursor"
	case "WINDSURF":
		command = "windsurf"
	case "ZED":
		command = "zed"
	case "NEOVIM":
		command = "nvim"
	}

	cmd := exec.Command(command, repo.Path)
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to open editor", "command", command, "error", err)
		errorWithData(w, http.StatusInternalServerError, "failed to open editor", map[string]string{"error": err.Error()})
		return
	}

	success(w, map[string]any{
		"opened": true,
		"path":   repo.Path,
	})
}
