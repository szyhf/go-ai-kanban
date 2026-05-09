package repository

import (
	"database/sql"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// ProjectRepo provides data access for projects.
type ProjectRepo struct {
	db *sql.DB
}

// NewProjectRepo creates a new ProjectRepo.
func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// FindAll returns all projects ordered by name.
func (r *ProjectRepo) FindAll() ([]domain.Project, error) {
	rows, err := r.db.Query(`
		SELECT id, name, default_agent_working_dir, remote_project_id,
		       created_at, updated_at
		FROM projects
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("find all projects: %w", err)
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		var remoteID []byte
		if err := rows.Scan(
			&p.ID, &p.Name, &p.DefaultAgentWorkingDir, &remoteID,
			timeScanner{&p.CreatedAt}, timeScanner{&p.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		if remoteID != nil {
			var uid domain.UUID
			copy(uid[:], remoteID)
			p.RemoteProjectID = &uid
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// FindByID returns a project by its ID.
func (r *ProjectRepo) FindByID(id domain.UUID) (*domain.Project, error) {
	var p domain.Project
	var remoteID []byte
	err := r.db.QueryRow(`
		SELECT id, name, default_agent_working_dir, remote_project_id,
		       created_at, updated_at
		FROM projects
		WHERE id = ?
	`, id[:]).Scan(
		&p.ID, &p.Name, &p.DefaultAgentWorkingDir, &remoteID,
		timeScanner{&p.CreatedAt}, timeScanner{&p.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find project by id: %w", err)
	}
	if remoteID != nil {
		var uid domain.UUID
		copy(uid[:], remoteID)
		p.RemoteProjectID = &uid
	}
	return &p, nil
}

// Create creates a new project.
func (r *ProjectRepo) Create(p *domain.Project) error {
	_, err := r.db.Exec(`
		INSERT INTO projects (id, name, default_agent_working_dir, remote_project_id)
		VALUES (?, ?, ?, ?)
	`, p.ID[:], p.Name, nullString(p.DefaultAgentWorkingDir), nullUUID(p.RemoteProjectID))
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// Update updates an existing project.
func (r *ProjectRepo) Update(p *domain.Project) error {
	_, err := r.db.Exec(`
		UPDATE projects
		SET name = ?, default_agent_working_dir = ?, remote_project_id = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, p.Name, nullString(p.DefaultAgentWorkingDir), nullUUID(p.RemoteProjectID), p.ID[:])
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// Delete deletes a project by ID.
func (r *ProjectRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM projects WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
