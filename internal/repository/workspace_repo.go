package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// WorkspaceRepo provides data access for workspaces.
type WorkspaceRepo struct {
	db *sql.DB
}

// NewWorkspaceRepo creates a new WorkspaceRepo.
func NewWorkspaceRepo(db *sql.DB) *WorkspaceRepo {
	return &WorkspaceRepo{db: db}
}

// scanWorkspace scans a full workspace row from the current row position.
func scanWorkspace(sc interface{ Scan(...interface{}) error }) (domain.Workspace, error) {
	var w domain.Workspace
	var taskID []byte
	if err := sc.Scan(
		&w.ID, &taskID, &w.ContainerRef, &w.Branch,
		nullTimeScanner{&w.SetupCompletedAt}, timeScanner{&w.CreatedAt}, timeScanner{&w.UpdatedAt},
		&w.Archived, &w.Pinned, &w.Name, &w.WorktreeDeleted,
	); err != nil {
		return w, fmt.Errorf("scan workspace: %w", err)
	}
	if taskID != nil {
		var uid domain.UUID
		copy(uid[:], taskID)
		w.TaskID = &uid
	}
	return w, nil
}

// FindAll returns all workspaces ordered by updated_at descending.
func (r *WorkspaceRepo) FindAll() ([]domain.Workspace, error) {
	rows, err := r.db.Query(`
		SELECT id, task_id, container_ref, branch,
		       setup_completed_at, created_at, updated_at,
		       archived, pinned, name, worktree_deleted
		FROM workspaces
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("find all workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []domain.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, rows.Err()
}

// FindAllWithStatus returns workspaces with computed is_running and is_errored
// flags derived from correlated subqueries over execution_processes and sessions.
// If archived is non-nil, results are filtered to match that archived state.
// If limit is non-nil, the result set is capped at that number.
func (r *WorkspaceRepo) FindAllWithStatus(archived *bool, limit *int) ([]domain.WorkspaceWithStatus, error) {
	query := `
		SELECT w.id, w.task_id, w.container_ref, w.branch,
		       w.setup_completed_at, w.created_at, w.updated_at,
		       w.archived, w.pinned, w.name, w.worktree_deleted,
		       EXISTS(
			       SELECT 1 FROM sessions s
			       JOIN execution_processes ep ON ep.session_id = s.id
			       WHERE s.workspace_id = w.id
			         AND ep.status = 'running'
			         AND (ep.run_reason != 'devserver' OR ep.dropped = FALSE)
		       ) AS is_running,
		       EXISTS(
			       SELECT 1 FROM sessions s
			       JOIN execution_processes ep ON ep.session_id = s.id
			       WHERE s.workspace_id = w.id
			         AND ep.status = 'failed'
			         AND ep.run_reason = 'codingagent'
		       ) AS is_errored
		FROM workspaces w
	`

	var args []interface{}
	if archived != nil {
		query += ` WHERE w.archived = ?`
		args = append(args, *archived)
	}

	query += ` ORDER BY w.updated_at DESC`

	if limit != nil {
		query += ` LIMIT ?`
		args = append(args, *limit)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("find workspaces with status: %w", err)
	}
	defer rows.Close()

	var results []domain.WorkspaceWithStatus
	for rows.Next() {
		var w domain.WorkspaceWithStatus
		var taskID []byte
		if err := rows.Scan(
			&w.ID, &taskID, &w.ContainerRef, &w.Branch,
			nullTimeScanner{&w.SetupCompletedAt}, timeScanner{&w.CreatedAt}, timeScanner{&w.UpdatedAt},
			&w.Archived, &w.Pinned, &w.Name, &w.WorktreeDeleted,
			&w.IsRunning, &w.IsErrored,
		); err != nil {
			return nil, fmt.Errorf("scan workspace with status: %w", err)
		}
		if taskID != nil {
			var uid domain.UUID
			copy(uid[:], taskID)
			w.TaskID = &uid
		}
		results = append(results, w)
	}
	return results, rows.Err()
}

// FindByID returns a workspace by its ID. Returns nil, nil if not found.
func (r *WorkspaceRepo) FindByID(id domain.UUID) (*domain.Workspace, error) {
	row := r.db.QueryRow(`
		SELECT id, task_id, container_ref, branch,
		       setup_completed_at, created_at, updated_at,
		       archived, pinned, name, worktree_deleted
		FROM workspaces
		WHERE id = ?
	`, id[:])

	w, err := scanWorkspace(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find workspace by id: %w", err)
	}
	return &w, nil
}

// FindByBranch returns a workspace by its branch name. Returns nil, nil if not found.
func (r *WorkspaceRepo) FindByBranch(branch string) (*domain.Workspace, error) {
	row := r.db.QueryRow(`
		SELECT id, task_id, container_ref, branch,
		       setup_completed_at, created_at, updated_at,
		       archived, pinned, name, worktree_deleted
		FROM workspaces
		WHERE branch = ?
	`, branch)

	w, err := scanWorkspace(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find workspace by branch: %w", err)
	}
	return &w, nil
}

// Create inserts a new workspace.
func (r *WorkspaceRepo) Create(w *domain.Workspace) error {
	_, err := r.db.Exec(`
		INSERT INTO workspaces (id, task_id, container_ref, branch,
		                        setup_completed_at,
		                        archived, pinned, name, worktree_deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		w.ID[:], nullUUID(w.TaskID), nullString(w.ContainerRef), w.Branch,
		nullTime(w.SetupCompletedAt),
		w.Archived, w.Pinned, nullString(w.Name), w.WorktreeDeleted,
	)
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	return nil
}

// Update updates an existing workspace.
func (r *WorkspaceRepo) Update(w *domain.Workspace) error {
	_, err := r.db.Exec(`
		UPDATE workspaces
		SET task_id = ?, container_ref = ?, branch = ?,
		    setup_completed_at = ?, updated_at = datetime('now', 'subsec'),
		    archived = ?, pinned = ?, name = ?, worktree_deleted = ?
		WHERE id = ?
	`,
		nullUUID(w.TaskID), nullString(w.ContainerRef), w.Branch,
		nullTime(w.SetupCompletedAt),
		w.Archived, w.Pinned, nullString(w.Name), w.WorktreeDeleted,
		w.ID[:],
	)
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	return nil
}

// UpdateArchived sets the archived flag for a workspace.
func (r *WorkspaceRepo) UpdateArchived(id domain.UUID, archived bool) error {
	_, err := r.db.Exec(`
		UPDATE workspaces
		SET archived = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, archived, id[:])
	if err != nil {
		return fmt.Errorf("update workspace archived: %w", err)
	}
	return nil
}

// UpdateSetupCompleted sets setup_completed_at to the current timestamp.
func (r *WorkspaceRepo) UpdateSetupCompleted(id domain.UUID) error {
	_, err := r.db.Exec(`
		UPDATE workspaces
		SET setup_completed_at = datetime('now', 'subsec'),
		    updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, id[:])
	if err != nil {
		return fmt.Errorf("update workspace setup completed: %w", err)
	}
	return nil
}

// Delete deletes a workspace by ID.
func (r *WorkspaceRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM workspaces WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

// isNoRows checks if an error wraps sql.ErrNoRows.
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
