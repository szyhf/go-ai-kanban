package repository

import (
	"database/sql"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// TaskRepo provides data access for tasks.
type TaskRepo struct {
	db *sql.DB
}

// NewTaskRepo creates a new TaskRepo.
func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// FindByProjectID returns all tasks for a project ordered by created_at.
func (r *TaskRepo) FindByProjectID(projectID domain.UUID) ([]domain.Task, error) {
	rows, err := r.db.Query(`
		SELECT id, project_id, title, description, status,
		       parent_workspace_id, created_at, updated_at
		FROM tasks
		WHERE project_id = ?
		ORDER BY created_at
	`, projectID[:])
	if err != nil {
		return nil, fmt.Errorf("find tasks by project id: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		var t domain.Task
		var parentWorkspaceID []byte
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Title, &t.Description,
			&t.Status, &parentWorkspaceID,
			timeScanner{&t.CreatedAt}, timeScanner{&t.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if parentWorkspaceID != nil {
			var uid domain.UUID
			copy(uid[:], parentWorkspaceID)
			t.ParentWorkspaceID = &uid
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// FindByID returns a task by its ID. Returns nil, nil if not found.
func (r *TaskRepo) FindByID(id domain.UUID) (*domain.Task, error) {
	var t domain.Task
	var parentWorkspaceID []byte
	err := r.db.QueryRow(`
		SELECT id, project_id, title, description, status,
		       parent_workspace_id, created_at, updated_at
		FROM tasks
		WHERE id = ?
	`, id[:]).Scan(
		&t.ID, &t.ProjectID, &t.Title, &t.Description,
		&t.Status, &parentWorkspaceID,
		timeScanner{&t.CreatedAt}, timeScanner{&t.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find task by id: %w", err)
	}
	if parentWorkspaceID != nil {
		var uid domain.UUID
		copy(uid[:], parentWorkspaceID)
		t.ParentWorkspaceID = &uid
	}
	return &t, nil
}

// Create inserts a new task.
func (r *TaskRepo) Create(t *domain.Task) error {
	_, err := r.db.Exec(`
		INSERT INTO tasks (id, project_id, title, description, status,
		                   parent_workspace_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.ID[:], t.ProjectID[:], t.Title, nullString(t.Description),
		t.Status, nullUUID(t.ParentWorkspaceID),
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

// Update updates an existing task.
func (r *TaskRepo) Update(t *domain.Task) error {
	_, err := r.db.Exec(`
		UPDATE tasks
		SET title = ?, description = ?, status = ?,
		    parent_workspace_id = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`,
		t.Title, nullString(t.Description), t.Status,
		nullUUID(t.ParentWorkspaceID), t.ID[:],
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

// UpdateStatus updates only the status of a task.
func (r *TaskRepo) UpdateStatus(id domain.UUID, status domain.TaskStatus) error {
	_, err := r.db.Exec(`
		UPDATE tasks
		SET status = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, status, id[:])
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	return nil
}

// Delete deletes a task by ID.
func (r *TaskRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM tasks WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}
