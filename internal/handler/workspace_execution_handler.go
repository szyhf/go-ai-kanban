package handler

import (
	"log/slog"
	"net/http"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/executor"
)

// startDevServer handles POST /api/workspaces/{id}/execution/dev-server/start.
// Stops any existing dev servers, then starts dev server scripts for repos that have them.
func (h *Handler) startDevServer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	// Stop any existing running dev servers.
	sessions, _ := h.sessionRepo.FindByWorkspaceID(id)
	for _, sess := range sessions {
		processes, err := h.execRepo.FindBySessionID(sess.ID, false)
		if err != nil {
			continue
		}
		for _, proc := range processes {
			if proc.Status == domain.ExecStatusRunning && proc.RunReason == domain.RunReasonDevServer {
				if err := h.containerSvc.StopExecution(proc.ID); err != nil {
					slog.Warn("停止已有的 dev server", "error", err, "process_id", proc.ID)
				}
			}
		}
	}

	// Find repos with dev server scripts.
	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	var devServerRepos []domain.RepoWithTargetBranch
	for _, rwt := range repos {
		if rwt.DevServerScript != nil && *rwt.DevServerScript != "" {
			devServerRepos = append(devServerRepos, rwt)
		}
	}

	if len(devServerRepos) == 0 {
		errorWithData(w, http.StatusBadRequest, "no dev server script configured for any repository in this workspace", map[string]string{
			"type": "no_script_configured",
		})
		return
	}

	// Find or create a session for the dev server.
	var session *domain.Session
	for _, sess := range sessions {
		s := sess
		session = &s
		break
	}
	if session == nil {
		executorName := "dev-server"
		session = &domain.Session{
			ID:          domain.NewUUID(),
			WorkspaceID: id,
			Executor:    &executorName,
		}
		if err := h.sessionRepo.Create(session); err != nil {
			internalError(w, "failed to create dev server session: "+err.Error())
			return
		}
	}

	// Start a dev server process for each repo that has a script.
	var results []domain.ExecutionProcess
	for _, rwt := range devServerRepos {
		script := *rwt.DevServerScript
		workingDir := rwt.Name

		scriptAction := struct {
			Type       string  `json:"type"`
			Script     string  `json:"script"`
			WorkingDir *string `json:"working_dir,omitempty"`
		}{
			Type:       "ScriptRequest",
			Script:     script,
			WorkingDir: &workingDir,
		}
		rawAction, err := domain.BuildExecutorActionJSON(scriptAction)
		if err != nil {
			slog.Error("构建 dev server action", "error", err)
			continue
		}

		processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
			Workspace: *ws,
			Session:   *session,
			RawAction: rawAction,
			RunReason: domain.RunReasonDevServer,
		}, nil, nil)
		if err != nil {
			slog.Error("启动 dev server", "error", err, "repo", rwt.Name)
			continue
		}

		ep, err := h.execRepo.FindByID(processID)
		if err != nil || ep == nil {
			continue
		}
		results = append(results, *ep)
	}

	created(w, results)
}

// runCleanupScript handles POST /api/workspaces/{id}/execution/cleanup.
// Runs the cleanup script for repos in the workspace.
func (h *Handler) runCleanupScript(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	if h.hasRunningNonDevServerProcesses(id) {
		errorWithData(w, http.StatusConflict, "a process is already running", map[string]string{
			"type": "process_already_running",
		})
		return
	}

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	var cleanupRepos []domain.RepoWithTargetBranch
	for _, rwt := range repos {
		if rwt.CleanupScript != nil && *rwt.CleanupScript != "" {
			cleanupRepos = append(cleanupRepos, rwt)
		}
	}

	if len(cleanupRepos) == 0 {
		errorWithData(w, http.StatusBadRequest, "no cleanup script configured", map[string]string{
			"type": "no_script_configured",
		})
		return
	}

	session := h.findOrCreateSession(r, id)
	if session == nil {
		return
	}

	script := *cleanupRepos[0].CleanupScript
	workingDir := cleanupRepos[0].Name

	scriptAction := struct {
		Type       string  `json:"type"`
		Script     string  `json:"script"`
		WorkingDir *string `json:"working_dir,omitempty"`
	}{
		Type:       "ScriptRequest",
		Script:     script,
		WorkingDir: &workingDir,
	}
	rawAction, err := domain.BuildExecutorActionJSON(scriptAction)
	if err != nil {
		internalError(w, "failed to build cleanup action")
		return
	}

	processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
		Workspace: *ws,
		Session:   *session,
		RawAction: rawAction,
		RunReason: domain.RunReasonCleanupScript,
	}, nil, nil)
	if err != nil {
		slog.Error("启动 cleanup 执行", "error", err)
		internalError(w, "failed to run cleanup script: "+err.Error())
		return
	}

	ep, err := h.execRepo.FindByID(processID)
	if err != nil || ep == nil {
		internalError(w, "cleanup started but failed to load process")
		return
	}

	created(w, ep)
}

// runArchiveScript handles POST /api/workspaces/{id}/execution/archive.
// Runs the archive script for repos in the workspace.
func (h *Handler) runArchiveScript(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ws, err := h.wsRepo.FindByID(id)
	if err != nil || ws == nil {
		notFound(w, "workspace not found")
		return
	}

	if h.hasRunningNonDevServerProcesses(id) {
		errorWithData(w, http.StatusConflict, "a process is already running", map[string]string{
			"type": "process_already_running",
		})
		return
	}

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	var archiveRepos []domain.RepoWithTargetBranch
	for _, rwt := range repos {
		if rwt.ArchiveScript != nil && *rwt.ArchiveScript != "" {
			archiveRepos = append(archiveRepos, rwt)
		}
	}

	if len(archiveRepos) == 0 {
		errorWithData(w, http.StatusBadRequest, "no archive script configured", map[string]string{
			"type": "no_script_configured",
		})
		return
	}

	session := h.findOrCreateSession(r, id)
	if session == nil {
		return
	}

	script := *archiveRepos[0].ArchiveScript
	workingDir := archiveRepos[0].Name

	scriptAction := struct {
		Type       string  `json:"type"`
		Script     string  `json:"script"`
		WorkingDir *string `json:"working_dir,omitempty"`
	}{
		Type:       "ScriptRequest",
		Script:     script,
		WorkingDir: &workingDir,
	}
	rawAction, err := domain.BuildExecutorActionJSON(scriptAction)
	if err != nil {
		internalError(w, "failed to build archive action")
		return
	}

	processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
		Workspace: *ws,
		Session:   *session,
		RawAction: rawAction,
		RunReason: domain.RunReasonArchiveScript,
	}, nil, nil)
	if err != nil {
		slog.Error("启动 archive 执行", "error", err)
		internalError(w, "failed to run archive script: "+err.Error())
		return
	}

	ep, err := h.execRepo.FindByID(processID)
	if err != nil || ep == nil {
		internalError(w, "archive started but failed to load process")
		return
	}

	created(w, ep)
}

// findOrCreateSession finds the first session for a workspace or creates one.
// Returns nil and writes error response if creation fails.
func (h *Handler) findOrCreateSession(r *http.Request, wsID domain.UUID) *domain.Session {
	sessions, err := h.sessionRepo.FindByWorkspaceID(wsID)
	if err == nil && len(sessions) > 0 {
		s := sessions[0]
		return &s
	}

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: wsID,
	}
	if err := h.sessionRepo.Create(session); err != nil {
		return nil
	}
	return session
}
