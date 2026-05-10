package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
)

const (
	// maxFileSizeBytes is the maximum allowed upload size (20 MB).
	maxFileSizeBytes = 20 * 1024 * 1024
	// vibeAttachmentsDir is the subdirectory inside worktrees for copied attachments.
	vibeAttachmentsDir = ".vibe-attachments"
)

// FileService provides business logic for file storage with SHA256 deduplication.
type FileService struct {
	cacheDir       string
	legacyCacheDir string
	attachRepo     *repository.AttachmentRepo
	wsAttachRepo   *repository.WorkspaceAttachmentRepo
}

// NewFileService creates a new FileService.
func NewFileService(cacheDir string, attachRepo *repository.AttachmentRepo, wsAttachRepo *repository.WorkspaceAttachmentRepo) *FileService {
	return &FileService{
		cacheDir:       filepath.Join(cacheDir, "attachments"),
		legacyCacheDir: filepath.Join(cacheDir, "images"),
		attachRepo:     attachRepo,
		wsAttachRepo:   wsAttachRepo,
	}
}

// EnsureCacheDir creates the cache directory if it doesn't exist.
func (s *FileService) EnsureCacheDir() error {
	return os.MkdirAll(s.cacheDir, 0o755)
}

// StoreFile stores file data with SHA256 deduplication.
// Returns the existing attachment if the same content is already stored.
func (s *FileService) StoreFile(data []byte, originalFilename string) (*domain.Attachment, error) {
	// Check size limit.
	if int64(len(data)) > maxFileSizeBytes {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(data), maxFileSizeBytes)
	}

	// Compute SHA256 hash.
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	// Check for duplicate.
	existing, err := s.attachRepo.FindByHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Detect MIME type.
	mimeType := detectMIME(data, originalFilename)

	// Generate file path.
	cleanName := sanitizeFilename(originalFilename)
	ext := filepath.Ext(cleanName)
	if ext == "" {
		ext = extensionFromMIME(mimeType)
	}
	baseName := strings.TrimSuffix(cleanName, ext)
	if baseName == "" {
		baseName = domain.NewUUID().String()[:8]
	}
	if len(baseName) > 50 {
		baseName = baseName[:50]
	}

	id := domain.NewUUID()
	newFilename := fmt.Sprintf("%s_%s%s", id.String()[:8], baseName, ext)
	filePath := filepath.Join(s.cacheDir, newFilename)

	// Write to disk.
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	// Insert DB record.
	attachment := &domain.Attachment{
		ID:           id,
		FilePath:     newFilename,
		OriginalName: originalFilename,
		MimeType:     mimeType,
		SizeBytes:    int64(len(data)),
		Hash:         hashStr,
	}
	if err := s.attachRepo.Create(attachment); err != nil {
		os.Remove(filePath) // cleanup on failure
		return nil, fmt.Errorf("create attachment record: %w", err)
	}

	return attachment, nil
}

// GetFile returns an attachment by ID.
func (s *FileService) GetFile(id domain.UUID) (*domain.Attachment, error) {
	return s.attachRepo.FindByID(id)
}

// DeleteFile removes an attachment by ID (disk + DB).
func (s *FileService) DeleteFile(id domain.UUID) error {
	attachment, err := s.attachRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("find attachment: %w", err)
	}
	if attachment == nil {
		return fmt.Errorf("attachment not found: %s", id)
	}

	// Delete from both cache locations.
	for _, dir := range []string{s.cacheDir, s.legacyCacheDir} {
		p := filepath.Join(dir, attachment.FilePath)
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err != nil {
				slog.Warn("删除附件文件失败", "path", p, "error", err)
			}
		}
	}

	return s.attachRepo.Delete(id)
}

// DeleteOrphanedFiles removes attachments not linked to any workspace.
func (s *FileService) DeleteOrphanedFiles() (int, int) {
	orphans, err := s.attachRepo.FindOrphanedFiles()
	if err != nil {
		slog.Error("查找孤立文件", "error", err)
		return 0, 0
	}

	deleted, failed := 0, 0
	for _, a := range orphans {
		if err := s.DeleteFile(a.ID); err != nil {
			slog.Warn("删除孤立文件", "id", a.ID, "error", err)
			failed++
		} else {
			deleted++
		}
	}
	return deleted, failed
}

// CopyFilesToWorktree copies the specified files to .vibe-attachments/ in the worktree.
func (s *FileService) CopyFilesToWorktree(worktreePath, agentWorkingDir string, fileIDs []domain.UUID) error {
	if len(fileIDs) == 0 {
		return nil
	}

	// Resolve target directory.
	targetDir := worktreePath
	if agentWorkingDir != "" {
		targetDir = agentWorkingDir
	}
	vibeDir := filepath.Join(targetDir, vibeAttachmentsDir)

	// Collect attachments.
	var attachments []domain.Attachment
	for _, id := range fileIDs {
		a, err := s.attachRepo.FindByID(id)
		if err != nil {
			slog.Warn("查找附件用于复制", "id", id, "error", err)
			continue
		}
		if a != nil {
			attachments = append(attachments, *a)
		}
	}

	if len(attachments) == 0 {
		return nil
	}

	// Fast path: check if all already exist.
	allExist := true
	for _, a := range attachments {
		dst := filepath.Join(vibeDir, a.OriginalName)
		if _, err := os.Stat(dst); err != nil {
			allExist = false
			break
		}
	}
	if allExist {
		return nil
	}

	// Create .vibe-attachments directory.
	if err := os.MkdirAll(vibeDir, 0o755); err != nil {
		return fmt.Errorf("create vibe-attachments dir: %w", err)
	}

	// Write .gitignore.
	gitignore := filepath.Join(vibeDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("*\n"), 0o644); err != nil {
		slog.Warn("写入 .gitignore", "error", err)
	}

	// Copy files.
	for _, a := range attachments {
		srcPath := s.resolveCachedPath(a.FilePath)
		if srcPath == "" {
			slog.Warn("磁盘上未找到附件文件", "file_path", a.FilePath)
			continue
		}

		dst := filepath.Join(vibeDir, a.OriginalName)
		if _, err := os.Stat(dst); err == nil {
			continue // already exists
		}

		srcFile, err := os.Open(srcPath)
		if err != nil {
			slog.Warn("打开源文件", "path", srcPath, "error", err)
			continue
		}
		dstFile, err := os.Create(dst)
		if err != nil {
			srcFile.Close()
			slog.Warn("创建目标文件", "path", dst, "error", err)
			continue
		}
		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			slog.Warn("复制文件", "src", srcPath, "dst", dst, "error", err)
		}
	}

	return nil
}

// CopyWorkspaceFilesToWorktree copies all files associated with a workspace to the worktree.
func (s *FileService) CopyWorkspaceFilesToWorktree(worktreePath, agentWorkingDir string, workspaceID domain.UUID) error {
	attachments, err := s.wsAttachRepo.FindByWorkspaceID(workspaceID)
	if err != nil {
		return fmt.Errorf("find workspace attachments: %w", err)
	}

	var ids []domain.UUID
	for _, a := range attachments {
		ids = append(ids, a.ID)
	}

	return s.CopyFilesToWorktree(worktreePath, agentWorkingDir, ids)
}

// GetAbsolutePath resolves the full filesystem path for an attachment.
func (s *FileService) GetAbsolutePath(attachment *domain.Attachment) string {
	if p := s.resolveCachedPath(attachment.FilePath); p != "" {
		return p
	}
	return filepath.Join(s.cacheDir, attachment.FilePath)
}

// resolveCachedPath finds the file in primary or legacy cache.
func (s *FileService) resolveCachedPath(filePath string) string {
	// Primary cache.
	p := filepath.Join(s.cacheDir, filePath)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// Legacy cache.
	p = filepath.Join(s.legacyCacheDir, filePath)
	if _, err := os.Stat(p); err == nil {
		slog.Debug("从旧缓存解析", "file_path", filePath)
		return p
	}
	return ""
}

// sanitizeFilename cleans a filename for safe storage.
func sanitizeFilename(name string) string {
	// Lowercase.
	s := strings.ToLower(name)
	// Replace spaces with underscores.
	s = strings.ReplaceAll(s, " ", "_")
	// Remove special characters (keep alphanumeric, dots, dashes, underscores).
	reg := regexp.MustCompile(`[^a-z0-9._-]`)
	s = reg.ReplaceAllString(s, "")
	// Trim leading dots to avoid hidden files.
	s = strings.TrimLeft(s, ".")
	if s == "" {
		s = "file"
	}
	return s
}

// detectMIME detects MIME type from content and filename.
func detectMIME(data []byte, filename string) *string {
	// Sniff from content.
	if len(data) > 0 {
		m := http.DetectContentType(data)
		if m != "application/octet-stream" {
			return &m
		}
	}
	// Fall back to extension.
	ext := filepath.Ext(filename)
	if ext != "" {
		m := mime.TypeByExtension(ext)
		if m != "" {
			return &m
		}
	}
	return nil
}

// extensionFromMIME returns a file extension for a MIME type.
func extensionFromMIME(mimeType *string) string {
	if mimeType == nil {
		return ""
	}
	exts, _ := mime.ExtensionsByType(*mimeType)
	if len(exts) > 0 {
		return exts[0]
	}
	return ""
}
