package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// registerProfileRoutes registers profile configuration routes.
func (h *Handler) registerProfileRoutes(r chi.Router) {
	r.Get("/", h.handleGetProfile)
	r.Put("/", h.handleSaveProfile)
}

// profileFilePath returns the path to the profile file.
func profileFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".vibe-kanban", "profile.md"), nil
}

// handleGetProfile handles GET /api/profiles.
func (h *Handler) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	path, err := profileFilePath()
	if err != nil {
		success(w, map[string]any{"content": "", "path": ""})
		return
	}

	content := ""
	data, err := os.ReadFile(path)
	if err == nil {
		content = string(data)
	}

	success(w, map[string]any{
		"content": content,
		"path":    path,
	})
}

// handleSaveProfile handles PUT /api/profiles.
func (h *Handler) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	path, err := profileFilePath()
	if err != nil {
		success(w, map[string]any{"content": "", "path": ""})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Warn("unable to create profile directory", "error", err)
		success(w, map[string]any{"content": req.Content, "path": path})
		return
	}

	if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
		slog.Warn("unable to write profile file", "error", err)
	}

	success(w, map[string]any{
		"content": req.Content,
		"path":    path,
	})
}
