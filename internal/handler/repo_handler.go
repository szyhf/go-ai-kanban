package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// registerRepoRoutes registers repo-related routes.
func (h *Handler) registerRepoRoutes(r chi.Router) {
	r.Get("/", h.listRepos)
	r.Post("/", h.registerRepo)
	r.Post("/init", h.initRepo)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getRepo)
		r.Put("/", h.updateRepo)
		r.Delete("/", h.deleteRepo)
		r.Get("/branches", h.listBranches)
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
