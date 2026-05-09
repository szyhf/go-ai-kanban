package repository

import (
	"database/sql"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// SessionRepo provides data access for sessions.
type SessionRepo struct {
	db *sql.DB
}

// NewSessionRepo creates a new SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// FindByWorkspaceID returns all sessions for a workspace, ordered by last used
// (most recent execution_processes.created_at) with fallback to session created_at.
func (r *SessionRepo) FindByWorkspaceID(workspaceID domain.UUID) ([]domain.Session, error) {
	rows, err := r.db.Query(`
		SELECT s.id, s.workspace_id, s.name, s.executor, s.agent_working_dir,
		       s.created_at, s.updated_at
		FROM sessions s
		LEFT JOIN (
			SELECT session_id, MAX(created_at) AS last_used
			FROM execution_processes
			GROUP BY session_id
		) latest_ep ON s.id = latest_ep.session_id
		WHERE s.workspace_id = ?
		ORDER BY COALESCE(latest_ep.last_used, s.created_at) DESC
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find sessions by workspace: %w", err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		var s domain.Session
		var name, executor, agentWorkingDir []byte
		if err := rows.Scan(
			&s.ID, &s.WorkspaceID, &name, &executor, &agentWorkingDir,
			timeScanner{&s.CreatedAt}, timeScanner{&s.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if name != nil {
			s.Name = new(string)
			*s.Name = string(name)
		}
		if executor != nil {
			s.Executor = new(string)
			*s.Executor = string(executor)
		}
		if agentWorkingDir != nil {
			s.AgentWorkingDir = new(string)
			*s.AgentWorkingDir = string(agentWorkingDir)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// FindByID returns a session by its ID. Returns nil, nil if not found.
func (r *SessionRepo) FindByID(id domain.UUID) (*domain.Session, error) {
	var s domain.Session
	var name, executor, agentWorkingDir []byte
	err := r.db.QueryRow(`
		SELECT id, workspace_id, name, executor, agent_working_dir,
		       created_at, updated_at
		FROM sessions
		WHERE id = ?
	`, id[:]).Scan(
		&s.ID, &s.WorkspaceID, &name, &executor, &agentWorkingDir,
		timeScanner{&s.CreatedAt}, timeScanner{&s.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find session by id: %w", err)
	}
	if name != nil {
		s.Name = new(string)
		*s.Name = string(name)
	}
	if executor != nil {
		s.Executor = new(string)
		*s.Executor = string(executor)
	}
	if agentWorkingDir != nil {
		s.AgentWorkingDir = new(string)
		*s.AgentWorkingDir = string(agentWorkingDir)
	}
	return &s, nil
}

// Create inserts a new session.
func (r *SessionRepo) Create(s *domain.Session) error {
	_, err := r.db.Exec(`
		INSERT INTO sessions (id, workspace_id, name, executor, agent_working_dir)
		VALUES (?, ?, ?, ?, ?)
	`, s.ID[:], s.WorkspaceID[:], nullString(s.Name), nullString(s.Executor), nullString(s.AgentWorkingDir))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// Update updates an existing session.
func (r *SessionRepo) Update(s *domain.Session) error {
	_, err := r.db.Exec(`
		UPDATE sessions
		SET name = ?, executor = ?, agent_working_dir = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, nullString(s.Name), nullString(s.Executor), nullString(s.AgentWorkingDir), s.ID[:])
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

// Delete deletes a session by ID.
func (r *SessionRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
