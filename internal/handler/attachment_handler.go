package handler

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// registerAttachmentRoutes registers attachment-related routes.
func (h *Handler) registerAttachmentRoutes(r chi.Router) {
	r.Post("/upload", h.uploadAttachment)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/file", h.serveAttachmentFile)
		r.Delete("/", h.deleteAttachment)
	})
}

// uploadAttachment handles POST /api/attachments/upload.
func (h *Handler) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	if !setupMultipartUpload(w, r, 20<<20) {
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, "missing file in form")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		internalError(w, "failed to read file")
		return
	}

	att, err := h.fileSvc.StoreFile(data, header.Filename)
	if err != nil {
		internalError(w, "failed to store file: "+err.Error())
		return
	}

	// Check for workspace_id to link the attachment.
	if wsIDStr := r.FormValue("workspace_id"); wsIDStr != "" {
		wsID, err := domain.ParseUUID(wsIDStr)
		if err == nil {
			_ = h.wsAttachRepo.Create(wsID, att.ID)
		}
	}

	created(w, att)
}

// serveAttachmentFile handles GET /api/attachments/{id}/file.
func (h *Handler) serveAttachmentFile(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	att, err := h.fileSvc.GetFile(id)
	if err != nil || att == nil {
		notFound(w, "attachment not found")
		return
	}

	absPath := h.fileSvc.GetAbsolutePath(att)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		notFound(w, "file not found on disk")
		return
	}

	// Set Content-Disposition for download.
	displayName := att.OriginalName
	if displayName == "" {
		displayName = filepath.Base(att.FilePath)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+displayName+"\"")

	if att.MimeType != nil && *att.MimeType != "" {
		w.Header().Set("Content-Type", *att.MimeType)
	}

	http.ServeFile(w, r, absPath)
}

// deleteAttachment handles DELETE /api/attachments/{id}.
func (h *Handler) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.fileSvc.DeleteFile(id); err != nil {
		internalError(w, "failed to delete attachment: "+err.Error())
		return
	}
	success(w, map[string]string{"status": "deleted"})
}

// uploadWorkspaceAttachment handles POST /api/workspaces/{id}/attachments/upload.
// Uploads a file and links it to the workspace.
func (h *Handler) uploadWorkspaceAttachment(w http.ResponseWriter, r *http.Request) {
	wsID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if !setupMultipartUpload(w, r, 20<<20) {
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			badRequest(w, "missing file in form")
			return
		}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		internalError(w, "failed to read file")
		return
	}

	att, err := h.fileSvc.StoreFile(data, header.Filename)
	if err != nil {
		internalError(w, "failed to store file: "+err.Error())
		return
	}

	// Link attachment to workspace.
	_ = h.wsAttachRepo.Create(wsID, att.ID)

	// Also link to session if session_id is provided.
	if sessionIDStr := r.FormValue("session_id"); sessionIDStr != "" {
		// Session linking is handled by the turn system if needed.
		_ = sessionIDStr
	}

	created(w, att)
}
