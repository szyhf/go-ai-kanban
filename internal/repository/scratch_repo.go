package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// ScratchRepo provides data access for scratchpad entries.
type ScratchRepo struct {
	db *sql.DB
}

// NewScratchRepo creates a new ScratchRepo.
func NewScratchRepo(db *sql.DB) *ScratchRepo {
	return &ScratchRepo{db: db}
}

// FindByIDAndType returns a scratch entry by its composite primary key (id, scratch_type).
// Returns nil, nil if not found.
func (r *ScratchRepo) FindByIDAndType(id domain.UUID, scratchType domain.ScratchType) (*domain.Scratch, error) {
	var s domain.Scratch
	var payloadBytes []byte
	err := r.db.QueryRow(`
		SELECT id, scratch_type, payload, created_at, updated_at
		FROM scratch
		WHERE id = ? AND scratch_type = ?
	`, id[:], scratchType).Scan(
		&s.ID, &s.Payload.Type, &payloadBytes,
		timeScanner{&s.CreatedAt}, timeScanner{&s.UpdatedAt},
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find scratch by id and type: %w", err)
	}
	s.Payload.Data = json.RawMessage(payloadBytes)
	return &s, nil
}

// FindAllByType returns all scratch entries of the given type.
func (r *ScratchRepo) FindAllByType(scratchType domain.ScratchType) ([]domain.Scratch, error) {
	rows, err := r.db.Query(`
		SELECT id, scratch_type, payload, created_at, updated_at
		FROM scratch
		WHERE scratch_type = ?
		ORDER BY updated_at DESC
	`, scratchType)
	if err != nil {
		return nil, fmt.Errorf("find all scratches by type: %w", err)
	}
	defer rows.Close()

	var scratches []domain.Scratch
	for rows.Next() {
		var s domain.Scratch
		var payloadBytes []byte
		if err := rows.Scan(
			&s.ID, &s.Payload.Type, &payloadBytes,
			timeScanner{&s.CreatedAt}, timeScanner{&s.UpdatedAt},
		); err != nil {
			return nil, fmt.Errorf("scan scratch: %w", err)
		}
		s.Payload.Data = json.RawMessage(payloadBytes)
		scratches = append(scratches, s)
	}
	return scratches, rows.Err()
}

// Upsert inserts or updates a scratch entry on composite PK conflict.
func (r *ScratchRepo) Upsert(s *domain.Scratch) error {
	_, err := r.db.Exec(`
		INSERT INTO scratch (id, scratch_type, payload, created_at, updated_at)
		VALUES (?, ?, ?, datetime('now','subsec'), datetime('now','subsec'))
		ON CONFLICT(id, scratch_type) DO UPDATE SET
			payload = excluded.payload,
			updated_at = datetime('now','subsec')
	`, s.ID[:], s.Payload.Type, []byte(s.Payload.Data))
	if err != nil {
		return fmt.Errorf("upsert scratch: %w", err)
	}
	return nil
}

// Delete removes a scratch entry by its composite primary key.
func (r *ScratchRepo) Delete(id domain.UUID, scratchType domain.ScratchType) error {
	_, err := r.db.Exec(`
		DELETE FROM scratch WHERE id = ? AND scratch_type = ?
	`, id[:], scratchType)
	if err != nil {
		return fmt.Errorf("delete scratch: %w", err)
	}
	return nil
}
