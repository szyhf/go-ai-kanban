package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// ExecutionProcessRepo provides data access for execution processes.
type ExecutionProcessRepo struct {
	db *sql.DB
}

// NewExecutionProcessRepo creates a new ExecutionProcessRepo.
func NewExecutionProcessRepo(db *sql.DB) *ExecutionProcessRepo {
	return &ExecutionProcessRepo{db: db}
}

// FindByID returns an execution process by its ID. Returns nil, nil if not found.
func (r *ExecutionProcessRepo) FindByID(id domain.UUID) (*domain.ExecutionProcess, error) {
	var ep domain.ExecutionProcess
	var executorAction []byte
	err := r.db.QueryRow(`
		SELECT id, session_id, run_reason, executor_action, status, exit_code,
		       dropped, started_at, completed_at, created_at, updated_at
		FROM execution_processes
		WHERE id = ?
	`, id[:]).Scan(
		&ep.ID, &ep.SessionID, &ep.RunReason, &executorAction, &ep.Status, &ep.ExitCode,
		&ep.Dropped, timeScanner{&ep.StartedAt}, nullTimeScanner{&ep.CompletedAt}, timeScanner{&ep.CreatedAt}, timeScanner{&ep.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find execution process by id: %w", err)
	}
	if executorAction != nil {
		ep.ExecutorAction = executorAction
	}
	return &ep, nil
}

// FindBySessionID returns execution processes for a session, ordered by created_at DESC.
// If includeDropped is false, only non-dropped processes are returned.
func (r *ExecutionProcessRepo) FindBySessionID(sessionID domain.UUID, includeDropped bool) ([]domain.ExecutionProcess, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, run_reason, executor_action, status, exit_code,
		       dropped, started_at, completed_at, created_at, updated_at
		FROM execution_processes
		WHERE session_id = ? AND (dropped = FALSE OR ?)
		ORDER BY created_at DESC
	`, sessionID[:], includeDropped)
	if err != nil {
		return nil, fmt.Errorf("find execution processes by session id: %w", err)
	}
	defer rows.Close()

	var result []domain.ExecutionProcess
	for rows.Next() {
		var ep domain.ExecutionProcess
		var executorAction []byte
		if err := rows.Scan(
			&ep.ID, &ep.SessionID, &ep.RunReason, &executorAction, &ep.Status, &ep.ExitCode,
			&ep.Dropped, timeScanner{&ep.StartedAt}, nullTimeScanner{&ep.CompletedAt}, timeScanner{&ep.CreatedAt}, timeScanner{&ep.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan execution process: %w", err)
		}
		if executorAction != nil {
			ep.ExecutorAction = executorAction
		}
		result = append(result, ep)
	}
	return result, rows.Err()
}

// Create inserts a new execution process. No transaction is used (intentional).
func (r *ExecutionProcessRepo) Create(ep *domain.ExecutionProcess) error {
	_, err := r.db.Exec(`
		INSERT INTO execution_processes (id, session_id, run_reason, executor_action, status, exit_code,
		                                 dropped, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ep.ID[:], ep.SessionID[:], ep.RunReason, ep.ExecutorAction, ep.Status, nullValue(ep.ExitCode),
		ep.Dropped, ep.StartedAt, nullTime(ep.CompletedAt))
	if err != nil {
		return fmt.Errorf("create execution process: %w", err)
	}
	return nil
}

// UpdateStatus updates the status and exit code of an execution process.
func (r *ExecutionProcessRepo) UpdateStatus(id domain.UUID, status domain.ExecStatus, exitCode *int64) error {
	_, err := r.db.Exec(`
		UPDATE execution_processes
		SET status = ?, exit_code = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, status, nullValue(exitCode), id[:])
	if err != nil {
		return fmt.Errorf("update execution process status: %w", err)
	}
	return nil
}

// UpdateDropped updates the dropped flag of an execution process.
func (r *ExecutionProcessRepo) UpdateDropped(id domain.UUID, dropped bool) error {
	_, err := r.db.Exec(`
		UPDATE execution_processes
		SET dropped = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, dropped, id[:])
	if err != nil {
		return fmt.Errorf("update execution process dropped: %w", err)
	}
	return nil
}

// FindLatestForWorkspaces returns the latest execution process info for each workspace.
// Returns an empty slice if workspaceIDs is empty.
func (r *ExecutionProcessRepo) FindLatestForWorkspaces(workspaceIDs []domain.UUID) ([]domain.LatestProcessInfo, error) {
	if len(workspaceIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(workspaceIDs))
	args := make([]any, len(workspaceIDs))
	for i, wid := range workspaceIDs {
		placeholders[i] = "?"
		args[i] = wid[:]
	}

	query := fmt.Sprintf(`
		SELECT ws_id, ep_id, s_id, status, completed_at FROM (
			SELECT s.workspace_id as ws_id, ep.id as ep_id, s.id as s_id, ep.status, ep.completed_at,
				ROW_NUMBER() OVER (PARTITION BY s.workspace_id ORDER BY ep.created_at DESC) as rn
			FROM execution_processes ep
			JOIN sessions s ON s.id = ep.session_id
			WHERE s.workspace_id IN (%s)
		) WHERE rn = 1
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("find latest execution processes for workspaces: %w", err)
	}
	defer rows.Close()

	var result []domain.LatestProcessInfo
	for rows.Next() {
		var info domain.LatestProcessInfo
		if err := rows.Scan(
			&info.WorkspaceID, &info.ExecutionProcessID, &info.SessionID,
			&info.Status, nullTimeScanner{&info.CompletedAt},
		); err != nil {
			return nil, fmt.Errorf("scan latest process info: %w", err)
		}
		result = append(result, info)
	}
	return result, rows.Err()
}

// scanExecutionProcess scans a full execution process row from a query result.
func scanExecutionProcess(row Row, ep *domain.ExecutionProcess) error {
	var executorAction []byte
	err := row.Scan(
		&ep.ID, &ep.SessionID, &ep.RunReason, &executorAction, &ep.Status, &ep.ExitCode,
		&ep.Dropped, timeScanner{&ep.StartedAt}, nullTimeScanner{&ep.CompletedAt}, timeScanner{&ep.CreatedAt}, timeScanner{&ep.UpdatedAt},
	)
	if err != nil {
		return err
	}
	if executorAction != nil {
		ep.ExecutorAction = executorAction
	}
	return nil
}

// ExecutionProcessRepoStateRepo provides data access for execution process repo states.
type ExecutionProcessRepoStateRepo struct {
	db *sql.DB
}

// NewExecutionProcessRepoStateRepo creates a new ExecutionProcessRepoStateRepo.
func NewExecutionProcessRepoStateRepo(db *sql.DB) *ExecutionProcessRepoStateRepo {
	return &ExecutionProcessRepoStateRepo{db: db}
}

// Create inserts a new execution process repo state.
func (r *ExecutionProcessRepoStateRepo) Create(state *domain.ExecutionProcessRepoState) error {
	_, err := r.db.Exec(`
		INSERT INTO execution_process_repo_states (id, execution_process_id, repo_id,
		                                           before_head_commit, after_head_commit, merge_commit)
		VALUES (?, ?, ?, ?, ?, ?)
	`, state.ID[:], state.ExecutionProcessID[:], state.RepoID[:],
		nullValue(state.BeforeHeadCommit), nullValue(state.AfterHeadCommit), nullValue(state.MergeCommit))
	if err != nil {
		return fmt.Errorf("create execution process repo state: %w", err)
	}
	return nil
}

// FindByExecutionProcessID returns all repo states for an execution process.
func (r *ExecutionProcessRepoStateRepo) FindByExecutionProcessID(processID domain.UUID) ([]domain.ExecutionProcessRepoState, error) {
	rows, err := r.db.Query(`
		SELECT id, execution_process_id, repo_id, before_head_commit, after_head_commit,
		       merge_commit, created_at, updated_at
		FROM execution_process_repo_states
		WHERE execution_process_id = ?
	`, processID[:])
	if err != nil {
		return nil, fmt.Errorf("find repo states by execution process id: %w", err)
	}
	defer rows.Close()

	var result []domain.ExecutionProcessRepoState
	for rows.Next() {
		var state domain.ExecutionProcessRepoState
		if err := rows.Scan(
			&state.ID, &state.ExecutionProcessID, &state.RepoID,
			&state.BeforeHeadCommit, &state.AfterHeadCommit, &state.MergeCommit,
			timeScanner{&state.CreatedAt}, timeScanner{&state.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan execution process repo state: %w", err)
		}
		result = append(result, state)
	}
	return result, rows.Err()
}

// UpdateAfterHeadCommit updates the after_head_commit and updated_at fields.
func (r *ExecutionProcessRepoStateRepo) UpdateAfterHeadCommit(id domain.UUID, commit string) error {
	_, err := r.db.Exec(`
		UPDATE execution_process_repo_states
		SET after_head_commit = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, commit, id[:])
	if err != nil {
		return fmt.Errorf("update after head commit: %w", err)
	}
	return nil
}

// CodingAgentTurnRepo provides data access for coding agent turns.
type CodingAgentTurnRepo struct {
	db *sql.DB
}

// NewCodingAgentTurnRepo creates a new CodingAgentTurnRepo.
func NewCodingAgentTurnRepo(db *sql.DB) *CodingAgentTurnRepo {
	return &CodingAgentTurnRepo{db: db}
}

// FindByExecutionProcessID returns all turns for an execution process.
func (r *CodingAgentTurnRepo) FindByExecutionProcessID(processID domain.UUID) ([]domain.CodingAgentTurn, error) {
	rows, err := r.db.Query(`
		SELECT id, execution_process_id, agent_session_id, agent_message_id,
		       prompt, summary, seen, created_at, updated_at
		FROM coding_agent_turns
		WHERE execution_process_id = ?
		ORDER BY created_at ASC
	`, processID[:])
	if err != nil {
		return nil, fmt.Errorf("find coding agent turns by execution process id: %w", err)
	}
	defer rows.Close()

	var result []domain.CodingAgentTurn
	for rows.Next() {
		var turn domain.CodingAgentTurn
		if err := rows.Scan(
			&turn.ID, &turn.ExecutionProcessID, &turn.AgentSessionID, &turn.AgentMessageID,
			&turn.Prompt, &turn.Summary, &turn.Seen, timeScanner{&turn.CreatedAt}, timeScanner{&turn.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan coding agent turn: %w", err)
		}
		result = append(result, turn)
	}
	return result, rows.Err()
}

// Create inserts a new coding agent turn.
func (r *CodingAgentTurnRepo) Create(turn *domain.CodingAgentTurn) error {
	_, err := r.db.Exec(`
		INSERT INTO coding_agent_turns (id, execution_process_id, agent_session_id, agent_message_id,
		                                prompt, summary, seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, turn.ID[:], turn.ExecutionProcessID[:],
		nullValue(turn.AgentSessionID), nullValue(turn.AgentMessageID),
		nullValue(turn.Prompt), nullValue(turn.Summary),
		turn.Seen)
	if err != nil {
		return fmt.Errorf("create coding agent turn: %w", err)
	}
	return nil
}

// UpdateSummaryAndSeen updates the summary and seen fields of a coding agent turn.
func (r *CodingAgentTurnRepo) UpdateSummaryAndSeen(id domain.UUID, summary *string, seen bool) error {
	_, err := r.db.Exec(`
		UPDATE coding_agent_turns
		SET summary = ?, seen = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, nullValue(summary), seen, id[:])
	if err != nil {
		return fmt.Errorf("update coding agent turn summary and seen: %w", err)
	}
	return nil
}

// FindLatestResumeInfo finds the latest turn with agent_session_id for any process in a session.
// Returns nil, nil if no turn with agent_session_id is found.
func (r *CodingAgentTurnRepo) FindLatestResumeInfo(sessionID domain.UUID) (*domain.CodingAgentResumeInfo, error) {
	var agentSessionID, agentMessageID sql.NullString
	err := r.db.QueryRow(`
		SELECT cat.agent_session_id, cat.agent_message_id
		FROM coding_agent_turns cat
		JOIN execution_processes ep ON ep.id = cat.execution_process_id
		WHERE ep.session_id = ? AND cat.agent_session_id IS NOT NULL
		ORDER BY cat.created_at DESC
		LIMIT 1
	`, sessionID[:]).Scan(&agentSessionID, &agentMessageID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find latest resume info: %w", err)
	}
	if !agentSessionID.Valid {
		return nil, nil
	}
	info := &domain.CodingAgentResumeInfo{
		SessionID: agentSessionID.String,
	}
	if agentMessageID.Valid {
		info.MessageID = &agentMessageID.String
	}
	return info, nil
}

// FindFirstUserPrompt returns the first user prompt for a workspace.
// Returns nil, nil if no user prompt is found.
func (r *CodingAgentTurnRepo) FindFirstUserPrompt(workspaceID domain.UUID) (*string, error) {
	var prompt sql.NullString
	err := r.db.QueryRow(`
		SELECT cat.prompt
		FROM coding_agent_turns cat
		JOIN execution_processes ep ON ep.id = cat.execution_process_id
		JOIN sessions s ON s.id = ep.session_id
		WHERE s.workspace_id = ? AND cat.prompt IS NOT NULL
		ORDER BY cat.created_at ASC
		LIMIT 1
	`, workspaceID[:]).Scan(&prompt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find first user prompt: %w", err)
	}
	if !prompt.Valid {
		return nil, nil
	}
	return &prompt.String, nil
}

// MarkSeenByWorkspaceID marks all coding agent turns as seen for a workspace.
func (r *CodingAgentTurnRepo) MarkSeenByWorkspaceID(workspaceID domain.UUID) error {
	_, err := r.db.Exec(`
		UPDATE coding_agent_turns
		SET seen = 1, updated_at = datetime('now', 'subsec')
		WHERE execution_process_id IN (
			SELECT ep.id
			FROM execution_processes ep
			JOIN sessions s ON s.id = ep.session_id
			WHERE s.workspace_id = ?
		)
	`, workspaceID[:])
	if err != nil {
		return fmt.Errorf("mark turns seen by workspace: %w", err)
	}
	return nil
}

// FindLatestBySessionID returns the latest execution process for a session.
// Returns nil, nil if no process is found.
func (r *ExecutionProcessRepo) FindLatestBySessionID(sessionID domain.UUID) (*domain.ExecutionProcess, error) {
	var ep domain.ExecutionProcess
	var executorAction []byte
	err := r.db.QueryRow(`
		SELECT id, session_id, run_reason, executor_action, status, exit_code,
		       dropped, started_at, completed_at, created_at, updated_at
		FROM execution_processes
		WHERE session_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, sessionID[:]).Scan(
		&ep.ID, &ep.SessionID, &ep.RunReason, &executorAction, &ep.Status, &ep.ExitCode,
		&ep.Dropped, timeScanner{&ep.StartedAt}, nullTimeScanner{&ep.CompletedAt}, timeScanner{&ep.CreatedAt}, timeScanner{&ep.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find latest execution process by session: %w", err)
	}
	if executorAction != nil {
		ep.ExecutorAction = executorAction
	}
	return &ep, nil
}

// FindRunningByWorkspaceID returns all running processes for a workspace.
func (r *ExecutionProcessRepo) FindRunningByWorkspaceID(workspaceID domain.UUID) ([]domain.ExecutionProcess, error) {
	rows, err := r.db.Query(`
		SELECT ep.id, ep.session_id, ep.run_reason, ep.executor_action, ep.status, ep.exit_code,
		       ep.dropped, ep.started_at, ep.completed_at, ep.created_at, ep.updated_at
		FROM execution_processes ep
		JOIN sessions s ON s.id = ep.session_id
		WHERE s.workspace_id = ? AND ep.status = 'Running'
		ORDER BY ep.created_at DESC
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find running processes by workspace: %w", err)
	}
	defer rows.Close()

	var result []domain.ExecutionProcess
	for rows.Next() {
		var ep domain.ExecutionProcess
		var executorAction []byte
		if err := rows.Scan(
			&ep.ID, &ep.SessionID, &ep.RunReason, &executorAction, &ep.Status, &ep.ExitCode,
			&ep.Dropped, timeScanner{&ep.StartedAt}, nullTimeScanner{&ep.CompletedAt}, timeScanner{&ep.CreatedAt}, timeScanner{&ep.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan execution process: %w", err)
		}
		if executorAction != nil {
			ep.ExecutorAction = executorAction
		}
		result = append(result, ep)
	}
	return result, rows.Err()
}

// DropAtAndAfter marks all processes at and after the boundary process as dropped.
func (r *ExecutionProcessRepo) DropAtAndAfter(sessionID domain.UUID, boundaryProcessID domain.UUID) error {
	_, err := r.db.Exec(`
		UPDATE execution_processes
		SET dropped = 1, updated_at = datetime('now', 'subsec')
		WHERE session_id = ?
		  AND created_at >= (SELECT created_at FROM execution_processes WHERE id = ?)
	`, sessionID[:], boundaryProcessID[:])
	if err != nil {
		return fmt.Errorf("drop execution processes at and after: %w", err)
	}
	return nil
}
