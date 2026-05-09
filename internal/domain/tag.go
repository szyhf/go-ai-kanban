package domain

import "time"

// Tag represents a reusable tag/template for tasks.
// Matches Rust crates/db/src/models/tag.rs.
type Tag struct {
	ID        UUID      `json:"id"`
	TagName   string    `json:"tag_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateTag is the request DTO for creating a tag.
type CreateTag struct {
	TagName string `json:"tag_name"`
	Content string `json:"content"`
}

// UpdateTag is the request DTO for updating a tag.
type UpdateTag struct {
	TagName *string `json:"tag_name,omitempty"`
	Content *string `json:"content,omitempty"`
}
