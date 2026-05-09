package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
)

func setupFileService(t *testing.T) (*FileService, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := database.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	attachRepo := repository.NewAttachmentRepo(db.DB)
	wsAttachRepo := repository.NewWorkspaceAttachmentRepo(db.DB)
	cacheDir := filepath.Join(dir, "cache")
	svc := NewFileService(cacheDir, attachRepo, wsAttachRepo)
	if err := svc.EnsureCacheDir(); err != nil {
		t.Fatalf("EnsureCacheDir: %v", err)
	}
	return svc, dir
}

func TestFileService_StoreFile(t *testing.T) {
	svc, _ := setupFileService(t)

	data := []byte("hello world")
	att, err := svc.StoreFile(data, "test.txt")
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}
	if att == nil {
		t.Fatal("expected non-nil attachment")
	}
	if att.OriginalName != "test.txt" {
		t.Errorf("OriginalName: got %q, want %q", att.OriginalName, "test.txt")
	}
	if att.SizeBytes != int64(len(data)) {
		t.Errorf("SizeBytes: got %d, want %d", att.SizeBytes, len(data))
	}
	if att.Hash == "" {
		t.Error("expected non-empty hash")
	}

	// Verify file exists on disk.
	fullPath := svc.GetAbsolutePath(att)
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("file not found at %q: %v", fullPath, err)
	}
}

func TestFileService_StoreFileDuplicate(t *testing.T) {
	svc, _ := setupFileService(t)

	data := []byte("duplicate content")
	att1, err := svc.StoreFile(data, "first.txt")
	if err != nil {
		t.Fatalf("StoreFile first: %v", err)
	}

	att2, err := svc.StoreFile(data, "second.txt")
	if err != nil {
		t.Fatalf("StoreFile second: %v", err)
	}

	// Should return the same attachment (dedup by hash).
	if att2.ID != att1.ID {
		t.Errorf("expected same ID for duplicate, got %s vs %s", att1.ID, att2.ID)
	}
}

func TestFileService_StoreFileTooLarge(t *testing.T) {
	svc, _ := setupFileService(t)

	largeData := make([]byte, maxFileSizeBytes+1)
	_, err := svc.StoreFile(largeData, "big.bin")
	if err == nil {
		t.Error("expected error for oversized file")
	}
}

func TestFileService_GetFile(t *testing.T) {
	svc, _ := setupFileService(t)

	data := []byte("get me")
	att, err := svc.StoreFile(data, "get.txt")
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	found, err := svc.GetFile(att.ID)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if found == nil || found.ID != att.ID {
		t.Error("GetFile mismatch")
	}
}

func TestFileService_DeleteFile(t *testing.T) {
	svc, _ := setupFileService(t)

	data := []byte("delete me")
	att, err := svc.StoreFile(data, "delete.txt")
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	if err := svc.DeleteFile(att.ID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	// Should be gone.
	found, err := svc.GetFile(att.ID)
	if err != nil {
		t.Fatalf("GetFile after delete: %v", err)
	}
	if found != nil {
		t.Error("expected nil after deletion")
	}
}

func TestFileService_DeleteNonExistent(t *testing.T) {
	svc, _ := setupFileService(t)

	err := svc.DeleteFile(domain.NewUUID())
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestFileService_CopyFilesToWorktree(t *testing.T) {
	svc, _ := setupFileService(t)

	data := []byte("copy me")
	att, err := svc.StoreFile(data, "copy.txt")
	if err != nil {
		t.Fatalf("StoreFile: %v", err)
	}

	worktreeDir := t.TempDir()
	err = svc.CopyFilesToWorktree(worktreeDir, "", []domain.UUID{att.ID})
	if err != nil {
		t.Fatalf("CopyFilesToWorktree: %v", err)
	}

	// Verify .vibe-attachments exists.
	vibeDir := filepath.Join(worktreeDir, vibeAttachmentsDir)
	if _, err := os.Stat(vibeDir); err != nil {
		t.Errorf(".vibe-attachments dir not created: %v", err)
	}

	// Verify file was copied.
	dst := filepath.Join(vibeDir, "copy.txt")
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("file not copied to %q: %v", dst, err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Hello World.txt", "hello_world.txt"},
		{"file with spaces.pdf", "file_with_spaces.pdf"},
		{"UPPER.CSV", "upper.csv"},
		{"special!@#chars.png", "specialchars.png"},
		{"..hidden", "hidden"},
		{"", "file"},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
