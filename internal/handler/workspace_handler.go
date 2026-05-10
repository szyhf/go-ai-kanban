package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/executor"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// registerWorkspaceRoutes registers workspace-related routes.
func (h *Handler) registerWorkspaceRoutes(r chi.Router) {
	r.Get("/", h.listWorkspaces)
	r.Post("/", h.createWorkspace)
	r.Post("/start", h.startWorkspace)
	r.Post("/from-pr", h.createWorkspaceFromPR)
	r.Post("/summaries", h.workspaceSummaries)
	r.Get("/streams/ws", h.handleWorkspaceStreamWS)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getWorkspace)
		r.Put("/", h.updateWorkspace)
		r.Put("/seen", h.markWorkspaceSeen)
		r.Delete("/", h.deleteWorkspace)
		r.Get("/messages/first", h.getFirstMessage)
		r.Post("/execution/stop", h.stopWorkspaceExecution)
		r.Post("/execution/dev-server/start", h.startDevServer)
		r.Post("/execution/cleanup", h.runCleanupScript)
		r.Post("/execution/archive", h.runArchiveScript)
		r.Post("/attachments/upload", h.uploadWorkspaceAttachment)
		r.Route("/git", func(r chi.Router) {
			r.Get("/status", h.gitStatus)
			r.Get("/diff/ws", h.handleDiffStreamWS)
			r.Post("/merge", h.gitMerge)
			r.Post("/push", h.gitPush)
			r.Post("/push/force", h.gitForcePush)
			r.Post("/rebase", h.gitRebase)
			r.Post("/rebase/continue", h.gitRebaseContinue)
			r.Post("/conflicts/abort", h.gitConflictsAbort)
			r.Put("/target-branch", h.changeTargetBranch)
			r.Put("/branch", h.renameBranch)
		})
		r.Get("/repos", h.listWorkspaceRepos)
		r.Post("/repos", h.addWorkspaceRepo)
		r.Route("/pull-requests", func(r chi.Router) {
			r.Post("/", h.createPR)
			r.Post("/attach", h.attachPR)
			r.Get("/comments", h.getPRComments)
		})
		r.Route("/integration", func(r chi.Router) {
			r.Post("/agent/setup", h.handleWorkspaceAgentSetup)
			r.Post("/editor/open", h.handleWorkspaceEditorOpen)
			r.Get("/editor/path", h.handleWorkspaceEditorPath)
			r.Post("/github/cli/setup", h.handleWorkspaceGHCLISetup)
		})
	})
}

// handleWorkspaceAgentSetup handles POST /api/workspaces/{id}/integration/agent/setup.
func (h *Handler) handleWorkspaceAgentSetup(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{})
}

// handleWorkspaceEditorOpen handles POST /api/workspaces/{id}/integration/editor/open.
func (h *Handler) handleWorkspaceEditorOpen(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{})
}

// handleWorkspaceEditorPath handles GET /api/workspaces/{id}/integration/editor/path.
func (h *Handler) handleWorkspaceEditorPath(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]string{"workspace_path": ""})
}

// handleWorkspaceGHCLISetup handles POST /api/workspaces/{id}/integration/github/cli/setup.
func (h *Handler) handleWorkspaceGHCLISetup(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{})
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

// startWorkspaceRequest is the request body for POST /api/workspaces/start.
type startWorkspaceRequest struct {
	Name           *string               `json:"name"`
	Repos          []startWorkspaceRepo  `json:"repos"`
	ExecutorConfig domain.ExecutorConfig `json:"executor_config"`
	Prompt         string                `json:"prompt"`
	AttachmentIDs  []string              `json:"attachment_ids"`
}

type startWorkspaceRepo struct {
	RepoID       string `json:"repo_id"`
	TargetBranch string `json:"target_branch"`
}

// startWorkspace handles POST /api/workspaces/start.
// Creates a workspace, session, and starts execution in one call.
func (h *Handler) startWorkspace(w http.ResponseWriter, r *http.Request) {
	var req startWorkspaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Prompt == "" {
		badRequest(w, "prompt is required")
		return
	}
	if len(req.Repos) == 0 {
		badRequest(w, "repos is required")
		return
	}

	// Generate unique branch name.
	branchName := generateBranchName(req.Name)

	// 1. Create workspace.
	now := time.Now().UTC().Truncate(time.Microsecond)
	ws := &domain.Workspace{
		ID:        domain.NewUUID(),
		Branch:    branchName,
		Name:      req.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.wsRepo.Create(ws); err != nil {
		internalError(w, "failed to create workspace: "+err.Error())
		return
	}

	// 2. Add repos.
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
		if err := h.wsRepoRepo.CreateMany(ws.ID, wsRepos); err != nil {
			slog.Warn("failed to add repos to workspace", "error", err)
		}
	}

	// 3. Create session.
	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: ws.ID,
		Name:        req.Name,
	}
	if err := h.sessionRepo.Create(session); err != nil {
		internalError(w, "failed to create session: "+err.Error())
		return
	}

	// 4. Build executor action (initial request).
	initialAction := struct {
		Type           string                `json:"type"`
		Prompt         string                `json:"prompt"`
		ExecutorConfig domain.ExecutorConfig `json:"executor_config"`
	}{
		Type:           "CodingAgentInitialRequest",
		Prompt:         req.Prompt,
		ExecutorConfig: req.ExecutorConfig,
	}
	rawAction, err := domain.BuildExecutorActionJSON(initialAction)
	if err != nil {
		internalError(w, "failed to build executor action")
		return
	}

	// 5. Start execution.
	processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
		Workspace: *ws,
		Session:   *session,
		RawAction: rawAction,
		RunReason: domain.RunReasonCodingAgent,
	}, nil, nil)
	if err != nil {
		slog.Error("start execution", "error", err)
		internalError(w, "failed to start execution: "+err.Error())
		return
	}

	// 6. Load the created process for the response.
	ep, err := h.execRepo.FindByID(processID)
	if err != nil || ep == nil {
		slog.Error("find execution process after creation", "error", err, "process_id", processID)
		internalError(w, "execution started but failed to load process")
		return
	}

	// 7. Notify frontend.
	h.eventSvc.NotifyChange(service.HookTableWorkspaces, service.HookOpInsert, ws.ID)

	created(w, map[string]interface{}{
		"workspace":         ws,
		"execution_process": ep,
	})
}

// nonAlphaNumRegex matches any non-alphanumeric character.
var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// generateBranchName creates a unique branch name from an optional workspace name.
func generateBranchName(name *string) string {
	short := domain.NewUUID().String()[:8]
	suffix := "workspace"
	if name != nil && *name != "" {
		s := strings.ToLower(*name)
		s = nonAlphaNumRegex.ReplaceAllString(s, "-")
		s = strings.Trim(s, "-")
		if s != "" {
			suffix = s
			if len(suffix) > 40 {
				suffix = suffix[:40]
			}
		}
	}
	return fmt.Sprintf("workspace-%s-%s", short, suffix)
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

// markWorkspaceSeen handles PUT /api/workspaces/{id}/seen.
func (h *Handler) markWorkspaceSeen(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.turnRepo.MarkSeenByWorkspaceID(id); err != nil {
		internalError(w, "failed to mark seen: "+err.Error())
		return
	}

	success(w, map[string]string{"status": "seen"})
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

// getFirstMessage handles GET /api/workspaces/{id}/messages/first.
func (h *Handler) getFirstMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	prompt, err := h.turnRepo.FindFirstUserPrompt(id)
	if err != nil {
		internalError(w, "failed to get first message: "+err.Error())
		return
	}
	if prompt == nil {
		notFound(w, "no messages found")
		return
	}
	success(w, map[string]string{"prompt": *prompt})
}

// stopWorkspaceExecution handles POST /api/workspaces/{id}/execution/stop.
func (h *Handler) stopWorkspaceExecution(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	processes, err := h.execRepo.FindRunningByWorkspaceID(id)
	if err != nil {
		internalError(w, "failed to find running processes: "+err.Error())
		return
	}

	stopped := 0
	for _, proc := range processes {
		if err := h.containerSvc.StopExecution(proc.ID); err != nil {
			slog.Warn("stop execution", "error", err, "process_id", proc.ID)
		} else {
			stopped++
		}
	}

	success(w, map[string]interface{}{
		"status":  "stopped",
		"stopped": stopped,
	})
}

// workspaceSummaries handles POST /api/workspaces/summaries.
func (h *Handler) workspaceSummaries(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceIDs []string `json:"workspace_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	type workspaceSummary struct {
		WorkspaceID     string `json:"workspace_id"`
		LatestStatus    string `json:"latest_status"`
		ProcessCount    int    `json:"process_count"`
		FileChangeCount int    `json:"file_change_count"`
	}

	summaries := make([]workspaceSummary, 0, len(req.WorkspaceIDs))
	for _, wsIDStr := range req.WorkspaceIDs {
		wsID, err := domain.ParseUUID(wsIDStr)
		if err != nil {
			continue
		}

		ws, err := h.wsRepo.FindByID(wsID)
		if err != nil || ws == nil {
			continue
		}

		// Get sessions for this workspace.
		sessions, err := h.sessionRepo.FindByWorkspaceID(wsID)
		if err != nil {
			continue
		}

		summary := workspaceSummary{
			WorkspaceID: wsIDStr,
		}

		// Count processes and get latest status.
		for _, sess := range sessions {
			processes, err := h.execRepo.FindBySessionID(sess.ID, false)
			if err != nil {
				continue
			}
			summary.ProcessCount += len(processes)
			for _, proc := range processes {
				if summary.LatestStatus == "" {
					summary.LatestStatus = string(proc.Status)
				}
			}
		}

		summaries = append(summaries, summary)
	}

	success(w, summaries)
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
