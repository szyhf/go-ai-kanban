package handler

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// registerWorkspaceRoutes registers workspace-related routes.
func (h *Handler) registerWorkspaceRoutes(r chi.Router) {
	r.Get("/", h.listWorkspaces)
	r.Post("/", h.createWorkspace)
	r.Get("/streams/ws", h.handleWorkspaceStreamWS)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getWorkspace)
		r.Put("/", h.updateWorkspace)
		r.Delete("/", h.deleteWorkspace)
		r.Route("/git", func(r chi.Router) {
			r.Get("/status", h.gitStatus)
			r.Get("/diff/ws", h.handleDiffStreamWS)
			r.Post("/merge", h.gitMerge)
			r.Post("/push", h.gitPush)
			r.Post("/rebase", h.gitRebase)
			r.Put("/target-branch", h.changeTargetBranch)
			r.Put("/branch", h.renameBranch)
		})
		r.Get("/repos", h.listWorkspaceRepos)
		r.Post("/repos", h.addWorkspaceRepo)
	})
}

// listWorkspaces handles GET /api/workspaces.
func (h *Handler) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := h.wsRepo.FindAllWithStatus(nil, nil)
	if err != nil {
		internalError(w, "failed to list workspaces: "+err.Error())
		return
	}
	success(w, workspaces)
}

// createWorkspace handles POST /api/workspaces.
func (h *Handler) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Branch string `json:"branch"`
		Name   string `json:"name"`
		Repos  []struct {
			RepoID       string `json:"repo_id"`
			TargetBranch string `json:"target_branch"`
		} `json:"repos"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Branch == "" {
		badRequest(w, "branch is required")
		return
	}

	ws := &domain.Workspace{
		ID:     domain.NewUUID(),
		Branch: req.Branch,
	}
	if req.Name != "" {
		ws.Name = &req.Name
	}

	if err := h.wsRepo.Create(ws); err != nil {
		internalError(w, "failed to create workspace: "+err.Error())
		return
	}

	// Add repos if provided.
	if len(req.Repos) > 0 {
		var wsRepos []domain.CreateWorkspaceRepo
		for _, rr := range req.Repos {
			repoID, err := domain.ParseUUID(rr.RepoID)
			if err != nil {
				continue
			}
			wsRepos = append(wsRepos, domain.CreateWorkspaceRepo{
				RepoID:       repoID,
				TargetBranch: rr.TargetBranch,
			})
		}
		if len(wsRepos) > 0 {
			_ = h.wsRepoRepo.CreateMany(ws.ID, wsRepos)
		}
	}

	h.eventSvc.NotifyChange(service.HookTableWorkspaces, service.HookOpInsert, ws.ID)

	created(w, ws)
}

// getWorkspace handles GET /api/workspaces/{id}.
func (h *Handler) getWorkspace(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}
	success(w, ws)
}

// updateWorkspace handles PUT /api/workspaces/{id}.
func (h *Handler) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	var updates struct {
		Archived *bool   `json:"archived"`
		Pinned   *bool   `json:"pinned"`
		Name     *string `json:"name"`
	}
	if !decodeJSON(w, r, &updates) {
		return
	}

	if updates.Archived != nil {
		ws.Archived = *updates.Archived
	}
	if updates.Pinned != nil {
		ws.Pinned = *updates.Pinned
	}
	if updates.Name != nil {
		ws.Name = updates.Name
	}

	if err := h.wsRepo.Update(ws); err != nil {
		internalError(w, "failed to update workspace: "+err.Error())
		return
	}

	h.eventSvc.NotifyChange(service.HookTableWorkspaces, service.HookOpUpdate, id)
	success(w, ws)
}

// deleteWorkspace handles DELETE /api/workspaces/{id}.
func (h *Handler) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.wsRepo.Delete(id); err != nil {
		internalError(w, "failed to delete workspace: "+err.Error())
		return
	}

	h.eventSvc.NotifyDelete(service.HookTableWorkspaces, id)
	success(w, map[string]string{"status": "deleted"})
}

// gitStatus handles GET /api/workspaces/{id}/git/status.
func (h *Handler) gitStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	repos, err := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	if err != nil {
		internalError(w, "failed to load workspace repos: "+err.Error())
		return
	}

	type repoStatus struct {
		RepoID string `json:"repo_id"`
		Path   string `json:"path"`
		Branch string `json:"branch"`
		Ahead  int    `json:"ahead"`
		Behind int    `json:"behind"`
	}

	var statuses []repoStatus
	for _, rwt := range repos {
		ahead, behind, _ := h.gitSvc.GetBranchStatus(rwt.Path, ws.Branch, rwt.TargetBranch)
		statuses = append(statuses, repoStatus{
			RepoID: rwt.ID.String(),
			Path:   rwt.Path,
			Branch: rwt.TargetBranch,
			Ahead:  ahead,
			Behind: behind,
		})
	}

	success(w, statuses)
}

// gitMerge handles POST /api/workspaces/{id}/git/merge.
func (h *Handler) gitMerge(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	var req struct {
		RepoID string `json:"repo_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	var target *domain.RepoWithTargetBranch
	for _, rwt := range repos {
		if rwt.ID.String() == req.RepoID || rwt.Path == req.RepoID {
			t := rwt
			target = &t
			break
		}
	}
	if target == nil {
		badRequest(w, "repo not found in workspace")
		return
	}

	commitMsg := fmt.Sprintf("Merge %s into %s", ws.Branch, target.TargetBranch)
	mergeCommit, err := h.gitSvc.MergeChanges(target.Path, "", ws.Branch, target.TargetBranch, commitMsg)
	if err != nil {
		internalError(w, "merge failed: "+err.Error())
		return
	}

	success(w, map[string]string{
		"merge_commit": mergeCommit,
		"status":       "merged",
	})
}

// gitPush handles POST /api/workspaces/{id}/git/push.
func (h *Handler) gitPush(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	var req struct {
		Force bool `json:"force"`
	}
	_ = decodeJSON(w, r, &req)

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	var results []map[string]string
	for _, rwt := range repos {
		err := h.gitSvc.PushToRemote(rwt.Path, ws.Branch, req.Force)
		status := "pushed"
		errMsg := ""
		if err != nil {
			status = "error"
			errMsg = err.Error()
		}
		results = append(results, map[string]string{
			"repo_id": rwt.ID.String(),
			"status":  status,
			"error":   errMsg,
		})
	}

	success(w, results)
}

// gitRebase handles POST /api/workspaces/{id}/git/rebase.
func (h *Handler) gitRebase(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	var results []map[string]string
	for _, rwt := range repos {
		err := h.gitSvc.RebaseBranch(rwt.Path, "", rwt.TargetBranch, "", ws.Branch)
		status := "rebased"
		errMsg := ""
		if err != nil {
			status = "error"
			errMsg = err.Error()
		}
		results = append(results, map[string]string{
			"repo_id": rwt.ID.String(),
			"status":  status,
			"error":   errMsg,
		})
	}

	success(w, results)
}

// changeTargetBranch handles PUT /api/workspaces/{id}/git/target-branch.
func (h *Handler) changeTargetBranch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		RepoID       string `json:"repo_id"`
		TargetBranch string `json:"target_branch"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.RepoID == "" || req.TargetBranch == "" {
		badRequest(w, "repo_id and target_branch are required")
		return
	}

	repoID, err := domain.ParseUUID(req.RepoID)
	if err != nil {
		badRequest(w, "invalid repo_id")
		return
	}

	// Delete and recreate the workspace-repo link with new target branch.
	_ = h.wsRepoRepo.DeleteByWorkspaceAndRepo(id, repoID)
	_ = h.wsRepoRepo.CreateMany(id, []domain.CreateWorkspaceRepo{{
		RepoID:       repoID,
		TargetBranch: req.TargetBranch,
	}})

	success(w, map[string]string{"status": "updated"})
}

// renameBranch handles PUT /api/workspaces/{id}/git/branch.
func (h *Handler) renameBranch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		NewName string `json:"new_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.NewName == "" {
		badRequest(w, "new_name is required")
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	for _, rwt := range repos {
		_ = h.gitSvc.RenameLocalBranch(rwt.Path, ws.Branch, req.NewName)
	}

	ws.Branch = req.NewName
	_ = h.wsRepo.Update(ws)

	h.eventSvc.NotifyChange(service.HookTableWorkspaces, service.HookOpUpdate, id)
	success(w, ws)
}

// listWorkspaceRepos handles GET /api/workspaces/{id}/repos.
func (h *Handler) listWorkspaceRepos(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	repos, err := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	if err != nil {
		internalError(w, "failed to list workspace repos: "+err.Error())
		return
	}
	success(w, repos)
}

// addWorkspaceRepo handles POST /api/workspaces/{id}/repos.
func (h *Handler) addWorkspaceRepo(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		RepoID       string `json:"repo_id"`
		TargetBranch string `json:"target_branch"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.RepoID == "" {
		badRequest(w, "repo_id is required")
		return
	}

	repoID, err := domain.ParseUUID(req.RepoID)
	if err != nil {
		badRequest(w, "invalid repo_id")
		return
	}

	err = h.wsRepoRepo.CreateMany(id, []domain.CreateWorkspaceRepo{{
		RepoID:       repoID,
		TargetBranch: req.TargetBranch,
	}})
	if err != nil {
		internalError(w, "failed to add repo: "+err.Error())
		return
	}

	created(w, map[string]string{"status": "added"})
}
