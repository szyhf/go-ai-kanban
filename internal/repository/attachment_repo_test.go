package repository

import (
	"database/sql"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// createTestProjectRow inserts a minimal project row via raw SQL.
// Used when we just need a project FK without going through ProjectRepo.
func createTestProjectRow(t *testing.T, db *sql.DB) domain.UUID {
	t.Helper()
	id := domain.NewUUID()
	_, err := db.Exec(`INSERT INTO projects (id, name) VALUES (?, 'test-project')`, id[:])
	if err != nil {
		t.Fatalf("insert test project row: %v", err)
	}
	return id
}

// createTestTaskRow inserts a minimal task row via raw SQL.
// Used when we just need a task FK without going through TaskRepo.
func createTestTaskRow(t *testing.T, db *sql.DB) domain.UUID {
	t.Helper()
	projectID := createTestProjectRow(t, db)
	id := domain.NewUUID()
	_, err := db.Exec(`
		INSERT INTO tasks (id, project_id, title, status)
		VALUES (?, ?, 'test', 'todo')
	`, id[:], projectID[:])
	if err != nil {
		t.Fatalf("insert test task row: %v", err)
	}
	return id
}

func TestAttachmentRepo_Create_FindByID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewAttachmentRepo(db.DB)

	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/test.png",
		OriginalName: "test.png",
		MimeType:     domain.StringPtr("image/png"),
		SizeBytes:    1024,
		Hash:         "sha256abc123",
	}

	if err := repo.Create(att); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(att.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID returned nil, expected attachment")
	}

	if found.ID != att.ID {
		t.Errorf("ID: got %s, want %s", found.ID, att.ID)
	}
	if found.FilePath != att.FilePath {
		t.Errorf("FilePath: got %q, want %q", found.FilePath, att.FilePath)
	}
	if found.OriginalName != att.OriginalName {
		t.Errorf("OriginalName: got %q, want %q", found.OriginalName, att.OriginalName)
	}
	if found.MimeType == nil || *found.MimeType != "image/png" {
		t.Errorf("MimeType: got %v, want %q", found.MimeType, "image/png")
	}
	if found.SizeBytes != att.SizeBytes {
		t.Errorf("SizeBytes: got %d, want %d", found.SizeBytes, att.SizeBytes)
	}
	if found.Hash != att.Hash {
		t.Errorf("Hash: got %q, want %q", found.Hash, att.Hash)
	}
	if found.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if found.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestAttachmentRepo_FindByHash(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewAttachmentRepo(db.DB)

	hash := "sha256-def456-unique"
	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/hash-test.pdf",
		OriginalName: "hash-test.pdf",
		MimeType:     domain.StringPtr("application/pdf"),
		SizeBytes:    2048,
		Hash:         hash,
	}
	if err := repo.Create(att); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByHash(hash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found == nil {
		t.Fatal("FindByHash returned nil, expected attachment")
	}
	if found.ID != att.ID {
		t.Errorf("ID: got %s, want %s", found.ID, att.ID)
	}
	if found.Hash != hash {
		t.Errorf("Hash: got %q, want %q", found.Hash, hash)
	}
}

func TestAttachmentRepo_FindByID_NotFound(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewAttachmentRepo(db.DB)

	found, err := repo.FindByID(domain.NewUUID())
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found != nil {
		t.Error("expected nil for non-existent attachment")
	}
}

func TestAttachmentRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	repo := NewAttachmentRepo(db.DB)

	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/to-delete.png",
		OriginalName: "to-delete.png",
		SizeBytes:    512,
		Hash:         "hash-delete",
	}
	if err := repo.Create(att); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(att.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	found, err := repo.FindByID(att.ID)
	if err != nil {
		t.Fatalf("FindByID after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after delete")
	}
}

func TestWorkspaceAttachmentRepo_Create_FindByWorkspaceID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	attRepo := NewAttachmentRepo(db.DB)
	wsAttRepo := NewWorkspaceAttachmentRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)

	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/ws-att.png",
		OriginalName: "ws-att.png",
		MimeType:     domain.StringPtr("image/png"),
		SizeBytes:    2048,
		Hash:         "ws-hash-123",
	}
	if err := attRepo.Create(att); err != nil {
		t.Fatalf("Create attachment: %v", err)
	}

	if err := wsAttRepo.Create(wsID, att.ID); err != nil {
		t.Fatalf("Create workspace attachment: %v", err)
	}

	attachments, err := wsAttRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("FindByWorkspaceID: got %d attachments, want 1", len(attachments))
	}
	if attachments[0].ID != att.ID {
		t.Errorf("ID: got %s, want %s", attachments[0].ID, att.ID)
	}
	if attachments[0].FilePath != att.FilePath {
		t.Errorf("FilePath: got %q, want %q", attachments[0].FilePath, att.FilePath)
	}
}

func TestWorkspaceAttachmentRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	attRepo := NewAttachmentRepo(db.DB)
	wsAttRepo := NewWorkspaceAttachmentRepo(db.DB)

	wsID := createTestWorkspace(t, db.DB)

	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/ws-del.png",
		OriginalName: "ws-del.png",
		SizeBytes:    100,
		Hash:         "ws-del-hash",
	}
	if err := attRepo.Create(att); err != nil {
		t.Fatalf("Create attachment: %v", err)
	}
	if err := wsAttRepo.Create(wsID, att.ID); err != nil {
		t.Fatalf("Create workspace attachment: %v", err)
	}

	if err := wsAttRepo.Delete(wsID, att.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	attachments, err := wsAttRepo.FindByWorkspaceID(wsID)
	if err != nil {
		t.Fatalf("FindByWorkspaceID after delete: %v", err)
	}
	if len(attachments) != 0 {
		t.Errorf("expected 0 attachments after delete, got %d", len(attachments))
	}
}

func TestTaskAttachmentRepo_Create_FindByTaskID(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	attRepo := NewAttachmentRepo(db.DB)
	taskAttRepo := NewTaskAttachmentRepo(db.DB)

	taskID := createTestTaskRow(t, db.DB)

	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/task-att.png",
		OriginalName: "task-att.png",
		MimeType:     domain.StringPtr("image/png"),
		SizeBytes:    4096,
		Hash:         "task-hash-456",
	}
	if err := attRepo.Create(att); err != nil {
		t.Fatalf("Create attachment: %v", err)
	}

	if err := taskAttRepo.Create(taskID, att.ID); err != nil {
		t.Fatalf("Create task attachment: %v", err)
	}

	attachments, err := taskAttRepo.FindByTaskID(taskID)
	if err != nil {
		t.Fatalf("FindByTaskID: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("FindByTaskID: got %d attachments, want 1", len(attachments))
	}
	if attachments[0].ID != att.ID {
		t.Errorf("ID: got %s, want %s", attachments[0].ID, att.ID)
	}
	if attachments[0].FilePath != att.FilePath {
		t.Errorf("FilePath: got %q, want %q", attachments[0].FilePath, att.FilePath)
	}
}

func TestTaskAttachmentRepo_Delete(t *testing.T) {
	db := testDB(t)
	t.Cleanup(func() { db.Close() })

	attRepo := NewAttachmentRepo(db.DB)
	taskAttRepo := NewTaskAttachmentRepo(db.DB)

	taskID := createTestTaskRow(t, db.DB)

	att := &domain.Attachment{
		ID:           domain.NewUUID(),
		FilePath:     "/uploads/task-del.png",
		OriginalName: "task-del.png",
		SizeBytes:    200,
		Hash:         "task-del-hash",
	}
	if err := attRepo.Create(att); err != nil {
		t.Fatalf("Create attachment: %v", err)
	}
	if err := taskAttRepo.Create(taskID, att.ID); err != nil {
		t.Fatalf("Create task attachment: %v", err)
	}

	if err := taskAttRepo.Delete(taskID, att.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	attachments, err := taskAttRepo.FindByTaskID(taskID)
	if err != nil {
		t.Fatalf("FindByTaskID after delete: %v", err)
	}
	if len(attachments) != 0 {
		t.Errorf("expected 0 attachments after delete, got %d", len(attachments))
	}
}
