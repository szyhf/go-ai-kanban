package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/git"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// ExecutionProcessRepos abstracts data access for execution processes.
type ExecutionProcessRepos interface {
	FindByID(id domain.UUID) (*domain.ExecutionProcess, error)
	Create(ep *domain.ExecutionProcess) error
	UpdateStatus(id domain.UUID, status domain.ExecStatus, exitCode *int64) error
}

// ExecutionRepoStateRepos abstracts data access for execution process repo states.
type ExecutionRepoStateRepos interface {
	Create(state *domain.ExecutionProcessRepoState) error
	FindByExecutionProcessID(processID domain.UUID) ([]domain.ExecutionProcessRepoState, error)
	UpdateAfterHeadCommit(id domain.UUID, commit string) error
}

// CodingAgentTurnRepos abstracts data access for coding agent turns.
type CodingAgentTurnRepos interface {
	Create(turn *domain.CodingAgentTurn) error
	FindByExecutionProcessID(processID domain.UUID) ([]domain.CodingAgentTurn, error)
}

// WorkspaceRepoRepos abstracts workspace-repo association queries.
type WorkspaceRepoRepos interface {
	FindByWorkspaceIDWithRepos(workspaceID domain.UUID) ([]domain.RepoWithTargetBranch, error)
}

// ContainerService orchestrates execution processes: spawn, monitor, commit, and chain.
type ContainerService struct {
	execProcRepo   ExecutionProcessRepos
	repoStateRepo  ExecutionRepoStateRepos
	turnRepo       CodingAgentTurnRepos
	wsRepoRepo     WorkspaceRepoRepos
	gitSvc         *git.Service
	processStore   *ProcessStore
	queueSvc       *service.QueuedMessageService
	logger         *slog.Logger
}

// NewContainerService creates a new ContainerService.
func NewContainerService(
	execProcRepo ExecutionProcessRepos,
	repoStateRepo ExecutionRepoStateRepos,
	turnRepo CodingAgentTurnRepos,
	wsRepoRepo WorkspaceRepoRepos,
	gitSvc *git.Service,
	processStore *ProcessStore,
	queueSvc *service.QueuedMessageService,
) *ContainerService {
	return &ContainerService{
		execProcRepo:  execProcRepo,
		repoStateRepo: repoStateRepo,
		turnRepo:      turnRepo,
		wsRepoRepo:    wsRepoRepo,
		gitSvc:        gitSvc,
		processStore:  processStore,
		queueSvc:      queueSvc,
		logger:        slog.Default(),
	}
}

// StartExecutionInput holds the parameters for starting an execution.
type StartExecutionInput struct {
	Workspace  domain.Workspace
	Session    domain.Session
	RawAction  json.RawMessage // Original JSON payload of the ExecutorAction
	WorkingDir string
	RunReason  domain.RunReason
}

// StartExecution creates a DB record, spawns the agent process, and begins monitoring.
// If executor is nil, only the DB record is created (no actual process is spawned).
func (svc *ContainerService) StartExecution(ctx context.Context, input StartExecutionInput, exec Executor, env *ExecutorEnv) (domain.UUID, error) {
	// Parse the action for routing decisions.
	action, err := domain.ParseExecutorAction(input.RawAction)
	if err != nil {
		return domain.UUID{}, fmt.Errorf("parse executor action: %w", err)
	}

	// 1. Create execution process DB record.
	processID := domain.NewUUID()
	now := time.Now().UTC().Truncate(time.Microsecond)

	ep := &domain.ExecutionProcess{
		ID:             processID,
		SessionID:      input.Session.ID,
		RunReason:      input.RunReason,
		ExecutorAction: input.RawAction,
		Status:         domain.ExecStatusRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := svc.execProcRepo.Create(ep); err != nil {
		return domain.UUID{}, fmt.Errorf("create execution process: %w", err)
	}

	// 2. Capture before_head_commit for each repo.
	wsRepos, err := svc.wsRepoRepo.FindByWorkspaceIDWithRepos(input.Workspace.ID)
	if err != nil {
		svc.logger.Warn("failed to find workspace repos for before-commit snapshot", "error", err)
	}
	for _, wr := range wsRepos {
		beforeCommit := svc.gitSvc.GetHeadCommit(wr.Path)
		var beforeSHA *string
		if beforeCommit != nil {
			beforeSHA = &beforeCommit.Hash
		}

		stateID := domain.NewUUID()
		state := &domain.ExecutionProcessRepoState{
			ID:                 stateID,
			ExecutionProcessID: processID,
			RepoID:             wr.ID,
			BeforeHeadCommit:   beforeSHA,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := svc.repoStateRepo.Create(state); err != nil {
			svc.logger.Warn("failed to create repo state", "repo_id", wr.ID, "error", err)
		}
	}

	// If no executor provided, create record only and mark as completed.
	if exec == nil {
		svc.logger.Info("execution started (no executor, record-only)", "process_id", processID)
		return processID, nil
	}

	// 3. Determine prompt from action.
	prompt, err := extractPrompt(input.RawAction)
	if err != nil {
		return domain.UUID{}, fmt.Errorf("extract prompt: %w", err)
	}

	// 4. Spawn the process.
	var proc *SpawnedProcess
	switch action.Type {
	case domain.ActionCodingAgentFollowUp:
		followUp, err := parseFollowUpRequest(input.RawAction)
		if err != nil {
			return domain.UUID{}, fmt.Errorf("parse follow-up request: %w", err)
		}
		proc, err = exec.SpawnFollowUp(ctx, input.WorkingDir, prompt, followUp.SessionID, followUp.ResetToMessageID, env)
		if err != nil {
			return domain.UUID{}, fmt.Errorf("spawn follow-up: %w", err)
		}
	default:
		proc, err = exec.Spawn(ctx, input.WorkingDir, prompt, env)
		if err != nil {
			return domain.UUID{}, fmt.Errorf("spawn: %w", err)
		}
	}

	// 5. Create MsgStore for log streaming.
	msgStore := service.NewMsgStore()

	// 6. Register in process store.
	svc.processStore.Add(processID, &ProcessEntry{
		Cmd:      proc.Cmd,
		Cancel:   proc.Cancel,
		MsgStore: msgStore,
		Done:     proc.Done,
	})

	// 7. Start log tracking goroutine.
	go svc.trackLogs(proc, msgStore)

	// 8. Start exit monitor goroutine.
	go svc.monitorExit(processID, input, msgStore, proc)

	svc.logger.Info("execution started",
		"process_id", processID,
		"session_id", input.Session.ID,
		"workspace_id", input.Workspace.ID,
		"pid", proc.Cmd.Process.Pid,
	)

	return processID, nil
}

// StopExecution gracefully stops a running execution process.
func (svc *ContainerService) StopExecution(processID domain.UUID) error {
	entry := svc.processStore.Get(processID)
	if entry == nil {
		return fmt.Errorf("process not found: %s", processID)
	}

	// Send interrupt first for graceful shutdown.
	if err := InterruptProcessGroup(entry.Cmd); err != nil {
		svc.logger.Warn("interrupt failed, killing", "process_id", processID, "error", err)
		if err := KillProcessGroup(entry.Cmd); err != nil {
			return fmt.Errorf("kill process group: %w", err)
		}
	}

	// Cancel the context.
	if entry.Cancel != nil {
		entry.Cancel()
	}

	// Update DB status.
	exitCode := int64(137) // SIGKILL exit code convention
	if err := svc.execProcRepo.UpdateStatus(processID, domain.ExecStatusKilled, &exitCode); err != nil {
		svc.logger.Error("failed to update killed status", "process_id", processID, "error", err)
	}

	// Push finished signal to msg store.
	entry.MsgStore.Push(service.NewFinishedLogMsg())

	// Clean up process store.
	svc.processStore.Remove(processID)

	svc.logger.Info("execution stopped", "process_id", processID)
	return nil
}

// GetMsgStore returns the MsgStore for a running process, or nil.
func (svc *ContainerService) GetMsgStore(processID domain.UUID) *service.MsgStore {
	entry := svc.processStore.Get(processID)
	if entry == nil {
		return nil
	}
	return entry.MsgStore
}

// monitorExit waits for the process to complete and performs cleanup.
func (svc *ContainerService) monitorExit(processID domain.UUID, input StartExecutionInput, msgStore *service.MsgStore, proc *SpawnedProcess) {
	result := <-proc.Done

	svc.logger.Info("execution exited",
		"process_id", processID,
		"exit_code", result.Code,
	)

	// Map exit code to status.
	status := exitCodeToStatus(result.Code)
	exitCode := int64(result.Code)

	if err := svc.execProcRepo.UpdateStatus(processID, status, &exitCode); err != nil {
		svc.logger.Error("failed to update exit status", "process_id", processID, "error", err)
	}

	// Push finished signal.
	msgStore.Push(service.NewFinishedLogMsg())

	// Auto-commit changes on success.
	if status == domain.ExecStatusCompleted {
		svc.autoCommitChanges(processID, input)
	}

	// Clean up from process store.
	svc.processStore.Remove(processID)

	// Check for queued follow-up messages.
	svc.checkQueuedFollowUp(input.Session.ID)
}

// trackLogs reads raw stdout/stderr lines and pushes them to the MsgStore.
func (svc *ContainerService) trackLogs(proc *SpawnedProcess, msgStore *service.MsgStore) {
	// Track stdout lines.
	go func() {
		for line := range proc.RawLines {
			msgStore.Push(service.NewStdoutLogMsg(line))
		}
	}()

	// Track stderr.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := proc.StderrPipe.Read(buf)
			if n > 0 {
				msgStore.Push(service.NewStderrLogMsg(string(buf[:n])))
			}
			if err != nil {
				if err != io.EOF {
					svc.logger.Debug("stderr read ended", "error", err)
				}
				return
			}
		}
	}()
}

// autoCommitChanges commits any uncommitted changes after a successful execution.
func (svc *ContainerService) autoCommitChanges(processID domain.UUID, input StartExecutionInput) {
	wsRepos, err := svc.wsRepoRepo.FindByWorkspaceIDWithRepos(input.Workspace.ID)
	if err != nil {
		svc.logger.Warn("failed to find workspace repos for auto-commit", "error", err)
		return
	}

	for _, wr := range wsRepos {
		commitMsg := fmt.Sprintf("Auto-commit from execution %s", processID)
		didCommit, err := svc.gitSvc.Commit(wr.Path, commitMsg)
		if err != nil {
			svc.logger.Warn("auto-commit failed", "repo", wr.Name, "error", err)
			continue
		}
		if didCommit {
			svc.logger.Info("auto-committed changes", "repo", wr.Name, "process_id", processID)

			// Update after_head_commit.
			head := svc.gitSvc.GetHeadCommit(wr.Path)
			if head != nil {
				states, err := svc.repoStateRepo.FindByExecutionProcessID(processID)
				if err != nil {
					svc.logger.Warn("failed to find repo states", "error", err)
					continue
				}
				for _, state := range states {
					if state.RepoID == wr.ID {
						if err := svc.repoStateRepo.UpdateAfterHeadCommit(state.ID, head.Hash); err != nil {
							svc.logger.Warn("failed to update after_head_commit", "error", err)
						}
						break
					}
				}
			}
		}
	}
}

// checkQueuedFollowUp checks if there's a queued message for the session and logs it.
func (svc *ContainerService) checkQueuedFollowUp(sessionID domain.UUID) {
	queued := svc.queueSvc.TakeQueued(sessionID)
	if queued != nil {
		svc.logger.Info("queued follow-up found for session",
			"session_id", sessionID,
			"queued_at", queued.QueuedAt,
		)
		// The actual follow-up spawning is handled by the handler layer
		// which calls StartExecution again with the queued message.
	}
}

// extractPrompt extracts the prompt from a raw ExecutorAction JSON payload.
func extractPrompt(rawAction json.RawMessage) (string, error) {
	inner := domain.UnwrapActionTyp(rawAction)

	// Try CodingAgentInitialRequest first.
	var initial domain.CodingAgentInitialRequest
	if err := json.Unmarshal(inner, &initial); err == nil && initial.Prompt != "" {
		return initial.Prompt, nil
	}

	// Try CodingAgentFollowUpRequest.
	var followUp domain.CodingAgentFollowUpRequest
	if err := json.Unmarshal(inner, &followUp); err == nil && followUp.Prompt != "" {
		return followUp.Prompt, nil
	}

	return "", fmt.Errorf("no prompt found in executor action")
}

// parseFollowUpRequest extracts follow-up parameters from raw JSON.
func parseFollowUpRequest(rawAction json.RawMessage) (*domain.CodingAgentFollowUpRequest, error) {
	inner := domain.UnwrapActionTyp(rawAction)
	var req domain.CodingAgentFollowUpRequest
	if err := json.Unmarshal(inner, &req); err != nil {
		return nil, fmt.Errorf("parse follow-up request: %w", err)
	}
	return &req, nil
}

// exitCodeToStatus maps a process exit code to an ExecStatus.
func exitCodeToStatus(code int) domain.ExecStatus {
	switch code {
	case 0:
		return domain.ExecStatusCompleted
	case 130: // SIGINT
		return domain.ExecStatusKilled
	case 137: // SIGKILL
		return domain.ExecStatusKilled
	default:
		return domain.ExecStatusFailed
	}
}
