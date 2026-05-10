package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/executor"
)

var errInvalidAction = errors.New("executor_action is required")

// registerExecutionRoutes registers execution process-related routes.
func (h *Handler) registerExecutionRoutes(r chi.Router) {
	// POST /api/execution-processes — create and start an execution.
	r.Post("/", h.createExecutionProcess)

	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getExecutionProcess)
		r.Post("/stop", h.stopExecutionProcess)
		r.Get("/repo-states", h.getRepoStates)
	})
}

// createExecutionRequest is the request body for creating an execution process.
type createExecutionRequest struct {
	SessionID      string                 `json:"session_id"`
	ExecutorAction map[string]interface{} `json:"executor_action"`
	RunReason      string                 `json:"run_reason"`
}

// createExecutionProcess handles POST /api/execution-processes.
// It creates a DB record and starts the execution via ContainerService.
func (h *Handler) createExecutionProcess(w http.ResponseWriter, r *http.Request) {
	if h.containerSvc == nil {
		internalError(w, "execution service not configured")
		return
	}

	var req createExecutionRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	sessionID, ok := parseUUIDFromString(w, req.SessionID)
	if !ok {
		return
	}

	// Look up the session.
	session, err := h.sessionRepo.FindByID(sessionID)
	if err != nil || session == nil {
		notFound(w, "session not found")
		return
	}

	// Look up the workspace.
	workspace, err := h.wsRepo.FindByID(session.WorkspaceID)
	if err != nil || workspace == nil {
		notFound(w, "workspace not found")
		return
	}

	// Marshal executor_action back to JSON.
	rawAction, err := marshalRawAction(req.ExecutorAction)
	if err != nil {
		badRequest(w, "invalid executor_action")
		return
	}

	runReason := domain.RunReason(req.RunReason)
	if runReason == "" {
		runReason = domain.RunReasonCodingAgent
	}

	// Determine working directory.
	workingDir := ""
	if session.AgentWorkingDir != nil {
		workingDir = *session.AgentWorkingDir
	}

	// Resolve executor from config.
	action, err := domain.ParseExecutorAction(rawAction)
	if err != nil {
		badRequest(w, "invalid executor action: "+err.Error())
		return
	}
	_ = action // Used later for routing; currently only Claude Code is supported.

	input := executor.StartExecutionInput{
		Workspace:  *workspace,
		Session:    *session,
		RawAction:  rawAction,
		WorkingDir: workingDir,
		RunReason:  runReason,
	}

	processID, err := h.containerSvc.StartExecution(r.Context(), input, nil, nil)
	if err != nil {
		internalError(w, "failed to start execution: "+err.Error())
		return
	}

	created(w, map[string]string{
		"id":     processID.String(),
		"status": string(domain.ExecStatusRunning),
	})
}

// getExecutionProcess handles GET /api/execution-processes/{id}.
func (h *Handler) getExecutionProcess(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ep, err := h.execRepo.FindByID(id)
	if err != nil || ep == nil {
		notFound(w, "execution process not found")
		return
	}
	success(w, ep)
}

// stopExecutionProcess handles POST /api/execution-processes/{id}/stop.
func (h *Handler) stopExecutionProcess(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	ep, err := h.execRepo.FindByID(id)
	if err != nil || ep == nil {
		notFound(w, "execution process not found")
		return
	}

	if ep.Status != domain.ExecStatusRunning {
		badRequest(w, "execution process is not running")
		return
	}

	// Use ContainerService to kill the process if available.
	if h.containerSvc != nil {
		if err := h.containerSvc.StopExecution(id); err != nil {
			internalError(w, "failed to stop execution process: "+err.Error())
			return
		}
	} else {
		// Fallback: just update DB status.
		exitCode := int64(137)
		if err := h.execRepo.UpdateStatus(id, domain.ExecStatusKilled, &exitCode); err != nil {
			internalError(w, "failed to stop execution process: "+err.Error())
			return
		}
	}

	success(w, map[string]string{"status": "stopped"})
}

// getRepoStates handles GET /api/execution-processes/{id}/repo-states.
func (h *Handler) getRepoStates(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	states, err := h.execStateRepo.FindByExecutionProcessID(id)
	if err != nil {
		internalError(w, "failed to get repo states: "+err.Error())
		return
	}
	success(w, states)
}

// parseUUIDFromString parses a UUID from a string field, writing error on failure.
func parseUUIDFromString(w http.ResponseWriter, s string) (domain.UUID, bool) {
	id, err := domain.ParseUUID(s)
	if err != nil {
		badRequest(w, "invalid UUID: "+s)
		return domain.UUID{}, false
	}
	return id, true
}

// marshalRawAction marshals a map back to JSON bytes.
func marshalRawAction(v map[string]interface{}) ([]byte, error) {
	if v == nil {
		return nil, errInvalidAction
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}
