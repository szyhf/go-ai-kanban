package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/executor"
)

// registerSessionRoutes registers session-related routes.
func (h *Handler) registerSessionRoutes(r chi.Router) {
	r.Get("/", h.listSessions)
	r.Post("/", h.createSession)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getSession)
		r.Put("/", h.updateSession)
		r.Post("/follow-up", h.handleFollowUp)
		r.Post("/reset", h.handleReset)
		r.Post("/review", h.startReview)
		r.Post("/setup", h.runSetupScript)
		r.Get("/queue", h.getQueue)
		r.Post("/queue", h.queueMessage)
		r.Delete("/queue", h.cancelQueued)
	})
}

// listSessions handles GET /api/sessions.
func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	wsIDStr := getQuery(r, "workspace_id")
	if wsIDStr == "" {
		badRequest(w, "workspace_id is required")
		return
	}

	wsID, err := domain.ParseUUID(wsIDStr)
	if err != nil {
		badRequest(w, "invalid workspace_id")
		return
	}

	sessions, err := h.sessionRepo.FindByWorkspaceID(wsID)
	if err != nil {
		internalError(w, "failed to list sessions: "+err.Error())
		return
	}
	success(w, sessions)
}

// createSession handles POST /api/sessions.
func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string  `json:"workspace_id"`
		Executor    *string `json:"executor"`
		Name        *string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.WorkspaceID == "" {
		badRequest(w, "workspace_id is required")
		return
	}

	wsID, err := domain.ParseUUID(req.WorkspaceID)
	if err != nil {
		badRequest(w, "invalid workspace_id")
		return
	}

	session := &domain.Session{
		ID:          domain.NewUUID(),
		WorkspaceID: wsID,
		Executor:    req.Executor,
		Name:        req.Name,
	}
	if err := h.sessionRepo.Create(session); err != nil {
		internalError(w, "failed to create session: "+err.Error())
		return
	}
	created(w, session)
}

// getSession handles GET /api/sessions/{id}.
func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	session, err := h.sessionRepo.FindByID(id)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}
	success(w, session)
}

// updateSession handles PUT /api/sessions/{id}.
func (h *Handler) updateSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	session, err := h.sessionRepo.FindByID(id)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	var req struct {
		Executor *string `json:"executor"`
		Name     *string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Executor != nil {
		session.Executor = req.Executor
	}
	if req.Name != nil {
		session.Name = req.Name
	}

	if err := h.sessionRepo.Update(session); err != nil {
		internalError(w, "failed to update session: "+err.Error())
		return
	}
	success(w, session)
}

// getQueue handles GET /api/sessions/{id}/queue.
func (h *Handler) getQueue(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	status := h.queueSvc.GetStatus(id)
	success(w, status)
}

// queueMessage handles POST /api/sessions/{id}/queue.
func (h *Handler) queueMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Prompt == "" {
		badRequest(w, "prompt is required")
		return
	}

	msg := h.queueSvc.QueueMessage(id, domain.DraftFollowUpData{Message: req.Prompt})
	created(w, msg)
}

// cancelQueued handles DELETE /api/sessions/{id}/queue.
func (h *Handler) cancelQueued(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	h.queueSvc.CancelQueued(id)
	success(w, map[string]string{"status": "cancelled"})
}

// followUpRequest matches the frontend's CreateFollowUpAttempt type.
type followUpRequest struct {
	Prompt          string                `json:"prompt"`
	ExecutorConfig  domain.ExecutorConfig `json:"executor_config"`
	RetryProcessID  *string               `json:"retry_process_id"`
	ForceWhenDirty  *bool                 `json:"force_when_dirty"`
	PerformGitReset *bool                 `json:"perform_git_reset"`
}

// handleFollowUp handles POST /api/sessions/{id}/follow-up.
func (h *Handler) handleFollowUp(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req followUpRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Prompt == "" {
		badRequest(w, "prompt is required")
		return
	}

	// Load session.
	session, err := h.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	// Load workspace.
	workspace, err := h.wsRepo.FindByID(session.WorkspaceID)
	if err != nil || workspace == nil {
		notFound(w, "workspace not found")
		return
	}

	// Handle retry: reset to the target process if specified.
	if req.RetryProcessID != nil && *req.RetryProcessID != "" {
		retryID, err := domain.ParseUUID(*req.RetryProcessID)
		if err != nil {
			badRequest(w, "invalid retry_process_id")
			return
		}
		performReset := true
		if req.PerformGitReset != nil {
			performReset = *req.PerformGitReset
		}
		if err := h.resetSessionToProcess(r.Context(), session, retryID, performReset); err != nil {
			slog.Error("重置 session 到 process", "error", err, "session_id", sessionID, "process_id", retryID)
			internalError(w, "failed to reset session: "+err.Error())
			return
		}
	}

	// Determine initial vs follow-up.
	var rawAction json.RawMessage

	resumeInfo, err := h.turnRepo.FindLatestResumeInfo(sessionID)
	if err != nil {
		slog.Error("查找最新恢复信息", "error", err)
		internalError(w, "failed to determine session state")
		return
	}

	if resumeInfo != nil {
		// Follow-up: resume an existing agent session.
		followUpAction := struct {
			Type             string                `json:"type"`
			Prompt           string                `json:"prompt"`
			SessionID        string                `json:"session_id"`
			ResetToMessageID *string               `json:"reset_to_message_id,omitempty"`
			ExecutorConfig   domain.ExecutorConfig `json:"executor_config"`
		}{
			Type:           "CodingAgentFollowUpRequest",
			Prompt:         req.Prompt,
			SessionID:      resumeInfo.SessionID,
			ExecutorConfig: req.ExecutorConfig,
		}
		if req.RetryProcessID != nil {
			followUpAction.ResetToMessageID = resumeInfo.MessageID
		}

		rawAction, err = domain.BuildExecutorActionJSON(followUpAction)
		if err != nil {
			internalError(w, "failed to build executor action")
			return
		}
	} else {
		// Initial: fresh coding agent invocation.
		initialAction := struct {
			Type           string                `json:"type"`
			Prompt         string                `json:"prompt"`
			ExecutorConfig domain.ExecutorConfig `json:"executor_config"`
		}{
			Type:           "CodingAgentInitialRequest",
			Prompt:         req.Prompt,
			ExecutorConfig: req.ExecutorConfig,
		}

		rawAction, err = domain.BuildExecutorActionJSON(initialAction)
		if err != nil {
			internalError(w, "failed to build executor action")
			return
		}
	}

	// Start execution.
	processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
		Workspace: *workspace,
		Session:   *session,
		RawAction: rawAction,
		RunReason: domain.RunReasonCodingAgent,
	}, nil, nil)
	if err != nil {
		slog.Error("启动执行", "error", err)
		internalError(w, "failed to start execution: "+err.Error())
		return
	}

	// Load the created process for the response.
	ep, err := h.execRepo.FindByID(processID)
	if err != nil || ep == nil {
		slog.Error("创建后查找 execution process", "error", err, "process_id", processID)
		internalError(w, "execution started but failed to load process")
		return
	}

	created(w, ep)
}

// handleReset handles POST /api/sessions/{id}/reset.
func (h *Handler) handleReset(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		ProcessID       string `json:"process_id"`
		PerformGitReset *bool  `json:"perform_git_reset"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProcessID == "" {
		badRequest(w, "process_id is required")
		return
	}

	processID, err := domain.ParseUUID(req.ProcessID)
	if err != nil {
		badRequest(w, "invalid process_id")
		return
	}

	// Load session to get workspace info.
	session, err := h.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	performReset := true
	if req.PerformGitReset != nil {
		performReset = *req.PerformGitReset
	}

	if err := h.resetSessionToProcess(r.Context(), session, processID, performReset); err != nil {
		slog.Error("重置 session", "error", err)
		internalError(w, "failed to reset session: "+err.Error())
		return
	}

	success(w, map[string]string{"status": "reset"})
}

// resetSessionToProcess performs a git reset and drops execution processes.
func (h *Handler) resetSessionToProcess(_ context.Context, session *domain.Session, processID domain.UUID, performGitReset bool) error {
	// Stop any running executions for this session.
	processes, err := h.execRepo.FindBySessionID(session.ID, false)
	if err != nil {
		return fmt.Errorf("find session processes: %w", err)
	}
	for _, proc := range processes {
		if proc.Status == domain.ExecStatusRunning {
			if err := h.containerSvc.StopExecution(proc.ID); err != nil {
				slog.Warn("重置时停止运行中的执行", "error", err, "process_id", proc.ID)
			}
		}
	}

	// If git reset is requested, reset each repo to the target branch.
	if performGitReset {
		wsRepos, err := h.wsRepoRepo.FindByWorkspaceIDWithRepos(session.WorkspaceID)
		if err != nil {
			slog.Warn("查找 workspace repos 用于重置", "error", err)
		} else {
			for _, wr := range wsRepos {
				if wr.TargetBranch != "" {
					if err := h.gitSvc.ResetHard(wr.Path, wr.TargetBranch); err != nil {
						slog.Warn("git reset hard", "repo", wr.Path, "branch", wr.TargetBranch, "error", err)
					}
				}
			}
		}
	}

	// Drop processes at and after the boundary.
	if err := h.execRepo.DropAtAndAfter(session.ID, processID); err != nil {
		return fmt.Errorf("drop processes: %w", err)
	}

	return nil
}

// startReview handles POST /api/sessions/{id}/review.
// Starts a code review execution by building a review prompt from the workspace's
// git diff and launching a coding agent.
func (h *Handler) startReview(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		ExecutorConfig         domain.ExecutorConfig `json:"executor_config"`
		AdditionalPrompt       *string               `json:"additional_prompt"`
		UseAllWorkspaceCommits bool                  `json:"use_all_workspace_commits"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	session, err := h.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	workspace, err := h.wsRepo.FindByID(session.WorkspaceID)
	if err != nil || workspace == nil {
		notFound(w, "workspace not found")
		return
	}

	// Check for running non-dev-server processes.
	if h.hasRunningNonDevServerProcesses(session.WorkspaceID) {
		errorWithData(w, http.StatusConflict, "a process is already running", map[string]string{
			"type": "process_already_running",
		})
		return
	}

	// Build review prompt.
	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(session.WorkspaceID)
	var promptParts []string
	for _, rwt := range repos {
		ahead, behind, _ := h.gitSvc.GetBranchStatus(rwt.Path, workspace.Branch, rwt.TargetBranch)
		if ahead > 0 || behind > 0 {
			promptParts = append(promptParts, fmt.Sprintf(
				"Repository: %s\nReview all changes from base commit of %s to HEAD on %s.",
				rwt.Name, rwt.TargetBranch, workspace.Branch,
			))
		}
	}

	if len(promptParts) == 0 {
		promptParts = append(promptParts, "Please review the code changes in this workspace.")
	}

	prompt := "Please review the code changes.\n\n" + strings.Join(promptParts, "\n\n")
	if req.AdditionalPrompt != nil && *req.AdditionalPrompt != "" {
		prompt += "\n\n" + *req.AdditionalPrompt
	}

	reviewAction := struct {
		Type           string                `json:"type"`
		Prompt         string                `json:"prompt"`
		ExecutorConfig domain.ExecutorConfig `json:"executor_config"`
	}{
		Type:           "ReviewRequest",
		Prompt:         prompt,
		ExecutorConfig: req.ExecutorConfig,
	}
	rawAction, err := domain.BuildExecutorActionJSON(reviewAction)
	if err != nil {
		internalError(w, "failed to build review action")
		return
	}

	processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
		Workspace: *workspace,
		Session:   *session,
		RawAction: rawAction,
		RunReason: domain.RunReasonCodingAgent,
	}, nil, nil)
	if err != nil {
		slog.Error("启动 review 执行", "error", err)
		internalError(w, "failed to start review: "+err.Error())
		return
	}

	ep, err := h.execRepo.FindByID(processID)
	if err != nil || ep == nil {
		internalError(w, "review started but failed to load process")
		return
	}

	created(w, ep)
}

// runSetupScript handles POST /api/sessions/{id}/setup.
// Runs the setup script for each repo in the workspace that has one configured.
func (h *Handler) runSetupScript(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	session, err := h.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	workspace, err := h.wsRepo.FindByID(session.WorkspaceID)
	if err != nil || workspace == nil {
		notFound(w, "workspace not found")
		return
	}

	// Check for running non-dev-server processes.
	if h.hasRunningNonDevServerProcesses(session.WorkspaceID) {
		errorWithData(w, http.StatusConflict, "a process is already running", map[string]string{
			"type": "process_already_running",
		})
		return
	}

	repos, _ := h.wsRepoRepo.FindByWorkspaceIDWithRepos(session.WorkspaceID)
	var scriptRepos []domain.RepoWithTargetBranch
	for _, rwt := range repos {
		if rwt.SetupScript != nil && *rwt.SetupScript != "" {
			scriptRepos = append(scriptRepos, rwt)
		}
	}

	if len(scriptRepos) == 0 {
		errorWithData(w, http.StatusBadRequest, "no setup script configured", map[string]string{
			"type": "no_script_configured",
		})
		return
	}

	// Build script action for the first repo with a setup script.
	script := *scriptRepos[0].SetupScript
	workingDir := scriptRepos[0].Name

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
		internalError(w, "failed to build setup action")
		return
	}

	processID, err := h.containerSvc.StartExecution(r.Context(), executor.StartExecutionInput{
		Workspace: *workspace,
		Session:   *session,
		RawAction: rawAction,
		RunReason: domain.RunReasonSetupScript,
	}, nil, nil)
	if err != nil {
		slog.Error("启动 setup 执行", "error", err)
		internalError(w, "failed to run setup script: "+err.Error())
		return
	}

	ep, err := h.execRepo.FindByID(processID)
	if err != nil || ep == nil {
		internalError(w, "setup started but failed to load process")
		return
	}

	created(w, ep)
}

// hasRunningNonDevServerProcesses checks if there are any running non-dev-server
// execution processes for the given workspace.
func (h *Handler) hasRunningNonDevServerProcesses(wsID domain.UUID) bool {
	sessions, err := h.sessionRepo.FindByWorkspaceID(wsID)
	if err != nil {
		return false
	}
	for _, sess := range sessions {
		processes, err := h.execRepo.FindBySessionID(sess.ID, false)
		if err != nil {
			continue
		}
		for _, proc := range processes {
			if proc.Status == domain.ExecStatusRunning && proc.RunReason != domain.RunReasonDevServer {
				return true
			}
		}
	}
	return false
}
