package repository

import (
	"database/sql"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// TagRepo provides data access for tags.
type TagRepo struct {
	db *sql.DB
}

// NewTagRepo creates a new TagRepo.
func NewTagRepo(db *sql.DB) *TagRepo {
	return &TagRepo{db: db}
}

// FindAll returns all tags ordered by tag_name.
func (r *TagRepo) FindAll() ([]domain.Tag, error) {
	rows, err := r.db.Query(`
		SELECT id, tag_name, content, created_at, updated_at
		FROM tags
		ORDER BY tag_name
	`)
	if err != nil {
		return nil, fmt.Errorf("find all tags: %w", err)
	}
	defer rows.Close()

	var tags []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.TagName, &t.Content, timeScanner{&t.CreatedAt}, timeScanner{&t.UpdatedAt}); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// FindByID returns a tag by its ID. Returns nil, nil if not found.
func (r *TagRepo) FindByID(id domain.UUID) (*domain.Tag, error) {
	var t domain.Tag
	err := r.db.QueryRow(`
		SELECT id, tag_name, content, created_at, updated_at
		FROM tags
		WHERE id = ?
	`, id[:]).Scan(&t.ID, &t.TagName, &t.Content, timeScanner{&t.CreatedAt}, timeScanner{&t.UpdatedAt})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find tag by id: %w", err)
	}
	return &t, nil
}

// Create inserts a new tag.
func (r *TagRepo) Create(t *domain.Tag) error {
	_, err := r.db.Exec(`
		INSERT INTO tags (id, tag_name, content)
		VALUES (?, ?, ?)
	`, t.ID[:], t.TagName, t.Content)
	if err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	return nil
}

// Update updates an existing tag.
func (r *TagRepo) Update(t *domain.Tag) error {
	_, err := r.db.Exec(`
		UPDATE tags
		SET tag_name = ?, content = ?, updated_at = datetime('now', 'subsec')
		WHERE id = ?
	`, t.TagName, t.Content, t.ID[:])
	if err != nil {
		return fmt.Errorf("update tag: %w", err)
	}
	return nil
}

// Delete deletes a tag by ID.
func (r *TagRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM tags WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}
