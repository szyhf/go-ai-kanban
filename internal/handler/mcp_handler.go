package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// registerMCPRoutes registers MCP configuration routes.
func (h *Handler) registerMCPRoutes(r chi.Router) {
	r.Get("/", h.handleGetMCPConfig)
	r.Post("/", h.handleSaveMCPConfig)
}

// mcpConfigPath returns the path to the MCP config file.
func mcpConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".vibe-kanban", "mcp.json"), nil
}

// handleGetMCPConfig handles GET /api/mcp-config.
func (h *Handler) handleGetMCPConfig(w http.ResponseWriter, r *http.Request) {
	path, err := mcpConfigPath()
	if err != nil || path == "" {
		success(w, map[string]any{"servers": []any{}})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		success(w, map[string]any{"servers": []any{}})
		return
	}

	// Parse and forward the stored config.
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		success(w, map[string]any{"servers": []any{}})
		return
	}

	success(w, config)
}

// handleSaveMCPConfig handles POST /api/mcp-config.
func (h *Handler) handleSaveMCPConfig(w http.ResponseWriter, r *http.Request) {
	path, err := mcpConfigPath()
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		badRequest(w, "failed to read body")
		return
	}

	// Validate it's valid JSON.
	var jsonTest any
	if err := json.Unmarshal(body, &jsonTest); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Warn("unable to create MCP config directory", "error", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Pretty format.
	var prettyJSON map[string]any
	json.Unmarshal(body, &prettyJSON)
	formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")

	if err := os.WriteFile(path, formatted, 0644); err != nil {
		slog.Warn("unable to write MCP config file", "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
