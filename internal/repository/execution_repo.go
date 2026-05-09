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

// Create inserts a new execution process. No transaction is used (intentional, matches Rust).
func (r *ExecutionProcessRepo) Create(ep *domain.ExecutionProcess) error {
	_, err := r.db.Exec(`
		INSERT INTO execution_processes (id, session_id, run_reason, executor_action, status, exit_code,
		                                 dropped, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ep.ID[:], ep.SessionID[:], ep.RunReason, ep.ExecutorAction, ep.Status, nullInt64(ep.ExitCode),
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
	`, status, nullInt64(exitCode), id[:])
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
	args := make([]interface{}, len(workspaceIDs))
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
func scanExecutionProcess(row interface{ Scan(...interface{}) error }, ep *domain.ExecutionProcess) error {
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
		nullString(state.BeforeHeadCommit), nullString(state.AfterHeadCommit), nullString(state.MergeCommit))
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
		nullString(turn.AgentSessionID), nullString(turn.AgentMessageID),
		nullString(turn.Prompt), nullString(turn.Summary),
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
	`, nullString(summary), seen, id[:])
	if err != nil {
		return fmt.Errorf("update coding agent turn summary and seen: %w", err)
	}
	return nil
}
