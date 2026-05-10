package domain

import "time"

// Attachment represents a stored file (originally "image", renamed to "attachment").
type Attachment struct {
	ID           UUID      `json:"id"`
	FilePath     string    `json:"file_path"`
	OriginalName string    `json:"original_name"`
	MimeType     *string   `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Hash         string    `json:"hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateAttachment is the request DTO for creating an attachment.
type CreateAttachment struct {
	FilePath     string  `json:"file_path"`
	OriginalName string  `json:"original_name"`
	MimeType     *string `json:"mime_type"`
	SizeBytes    int64   `json:"size_bytes"`
	Hash         string  `json:"hash"`
}

// WorkspaceAttachment is a junction between workspace and attachment.
type WorkspaceAttachment struct {
	ID           UUID      `json:"id"`
	WorkspaceID  UUID      `json:"workspace_id"`
	AttachmentID UUID      `json:"attachment_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// TaskAttachment is a junction between task and attachment.
type TaskAttachment struct {
	ID           UUID      `json:"id"`
	TaskID       UUID      `json:"task_id"`
	AttachmentID UUID      `json:"attachment_id"`
	CreatedAt    time.Time `json:"created_at"`
}
