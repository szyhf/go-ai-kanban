package repository

import (
	"database/sql"
	"fmt"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// AttachmentRepo provides data access for attachments.
type AttachmentRepo struct {
	db *sql.DB
}

// NewAttachmentRepo creates a new AttachmentRepo.
func NewAttachmentRepo(db *sql.DB) *AttachmentRepo {
	return &AttachmentRepo{db: db}
}

// scanAttachment scans a single attachment row into a domain.Attachment.
func scanAttachment(row interface {
	Scan(...interface{}) error
}, a *domain.Attachment) error {
	return row.Scan(
		&a.ID, &a.FilePath, &a.OriginalName, &a.MimeType,
		&a.SizeBytes, &a.Hash, timeScanner{&a.CreatedAt}, timeScanner{&a.UpdatedAt},
	)
}

// FindByID returns an attachment by its ID. Returns nil, nil if not found.
func (r *AttachmentRepo) FindByID(id domain.UUID) (*domain.Attachment, error) {
	var a domain.Attachment
	err := scanAttachment(r.db.QueryRow(`
		SELECT id, file_path, original_name, mime_type,
		       size_bytes, hash, created_at, updated_at
		FROM attachments
		WHERE id = ?
	`, id[:]), &a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find attachment by id: %w", err)
	}
	return &a, nil
}

// FindByHash returns an attachment by its content hash for deduplication.
// Returns nil, nil if not found.
func (r *AttachmentRepo) FindByHash(hash string) (*domain.Attachment, error) {
	var a domain.Attachment
	err := scanAttachment(r.db.QueryRow(`
		SELECT id, file_path, original_name, mime_type,
		       size_bytes, hash, created_at, updated_at
		FROM attachments
		WHERE hash = ?
	`, hash), &a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find attachment by hash: %w", err)
	}
	return &a, nil
}

// Create inserts a new attachment.
func (r *AttachmentRepo) Create(a *domain.Attachment) error {
	_, err := r.db.Exec(`
		INSERT INTO attachments (id, file_path, original_name, mime_type, size_bytes, hash)
		VALUES (?, ?, ?, ?, ?, ?)
	`, a.ID[:], a.FilePath, a.OriginalName, nullString(a.MimeType),
		a.SizeBytes, a.Hash)
	if err != nil {
		return fmt.Errorf("create attachment: %w", err)
	}
	return nil
}

// Delete removes an attachment by ID.
func (r *AttachmentRepo) Delete(id domain.UUID) error {
	_, err := r.db.Exec(`DELETE FROM attachments WHERE id = ?`, id[:])
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

// --- WorkspaceAttachmentRepo ---

// WorkspaceAttachmentRepo provides data access for the workspace-attachment junction.
type WorkspaceAttachmentRepo struct {
	db *sql.DB
}

// NewWorkspaceAttachmentRepo creates a new WorkspaceAttachmentRepo.
func NewWorkspaceAttachmentRepo(db *sql.DB) *WorkspaceAttachmentRepo {
	return &WorkspaceAttachmentRepo{db: db}
}

// FindByWorkspaceID returns all attachments linked to a workspace.
func (r *WorkspaceAttachmentRepo) FindByWorkspaceID(workspaceID domain.UUID) ([]domain.Attachment, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.file_path, a.original_name, a.mime_type,
		       a.size_bytes, a.hash, a.created_at, a.updated_at
		FROM attachments a
		JOIN workspace_attachments wa ON wa.attachment_id = a.id
		WHERE wa.workspace_id = ?
	`, workspaceID[:])
	if err != nil {
		return nil, fmt.Errorf("find attachments by workspace id: %w", err)
	}
	defer rows.Close()

	var attachments []domain.Attachment
	for rows.Next() {
		var a domain.Attachment
		if err := scanAttachment(rows, &a); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

// Create links an attachment to a workspace.
func (r *WorkspaceAttachmentRepo) Create(workspaceID, attachmentID domain.UUID) error {
	id := domain.NewUUID()
	_, err := r.db.Exec(`
		INSERT INTO workspace_attachments (id, workspace_id, attachment_id)
		VALUES (?, ?, ?)
	`, id[:], workspaceID[:], attachmentID[:])
	if err != nil {
		return fmt.Errorf("create workspace attachment: %w", err)
	}
	return nil
}

// Delete unlinks an attachment from a workspace.
func (r *WorkspaceAttachmentRepo) Delete(workspaceID, attachmentID domain.UUID) error {
	_, err := r.db.Exec(`
		DELETE FROM workspace_attachments
		WHERE workspace_id = ? AND attachment_id = ?
	`, workspaceID[:], attachmentID[:])
	if err != nil {
		return fmt.Errorf("delete workspace attachment: %w", err)
	}
	return nil
}

// --- TaskAttachmentRepo ---

// TaskAttachmentRepo provides data access for the task-attachment junction.
type TaskAttachmentRepo struct {
	db *sql.DB
}

// NewTaskAttachmentRepo creates a new TaskAttachmentRepo.
func NewTaskAttachmentRepo(db *sql.DB) *TaskAttachmentRepo {
	return &TaskAttachmentRepo{db: db}
}

// FindByTaskID returns all attachments linked to a task.
func (r *TaskAttachmentRepo) FindByTaskID(taskID domain.UUID) ([]domain.Attachment, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.file_path, a.original_name, a.mime_type,
		       a.size_bytes, a.hash, a.created_at, a.updated_at
		FROM attachments a
		JOIN task_attachments ta ON ta.attachment_id = a.id
		WHERE ta.task_id = ?
	`, taskID[:])
	if err != nil {
		return nil, fmt.Errorf("find attachments by task id: %w", err)
	}
	defer rows.Close()

	var attachments []domain.Attachment
	for rows.Next() {
		var a domain.Attachment
		if err := scanAttachment(rows, &a); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

// Create links an attachment to a task.
func (r *TaskAttachmentRepo) Create(taskID, attachmentID domain.UUID) error {
	id := domain.NewUUID()
	_, err := r.db.Exec(`
		INSERT INTO task_attachments (id, task_id, attachment_id)
		VALUES (?, ?, ?)
	`, id[:], taskID[:], attachmentID[:])
	if err != nil {
		return fmt.Errorf("create task attachment: %w", err)
	}
	return nil
}

// Delete unlinks an attachment from a task.
func (r *TaskAttachmentRepo) Delete(taskID, attachmentID domain.UUID) error {
	_, err := r.db.Exec(`
		DELETE FROM task_attachments
		WHERE task_id = ? AND attachment_id = ?
	`, taskID[:], attachmentID[:])
	if err != nil {
		return fmt.Errorf("delete task attachment: %w", err)
	}
	return nil
}
