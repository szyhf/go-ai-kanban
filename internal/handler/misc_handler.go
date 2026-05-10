package handler

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
)

// registerMiscRoutes registers miscellaneous stub routes that don't fit
// neatly into other handler categories.
func (h *Handler) registerMiscRoutes(r chi.Router) {
	// Editor availability check.
	r.Get("/editors/check-availability", h.handleEditorCheckAvailability)
	// Remote projects (cloud-only, stubbed for local mode).
	r.Get("/remote/projects", h.handleRemoteProjects)
	// Open remote editor (relay-only, stubbed).
	r.Post("/open-remote-editor/workspace", h.handleOpenRemoteEditor)
}

// handleEditorCheckAvailability handles GET /api/editors/check-availability.
func (h *Handler) handleEditorCheckAvailability(w http.ResponseWriter, r *http.Request) {
	editorType := getQuery(r, "editor_type")

	type editorInfo struct {
		Command string
	}

	editors := map[string]editorInfo{
		"VS_CODE":  {Command: "code"},
		"CURSOR":   {Command: "cursor"},
		"WINDSURF": {Command: "windsurf"},
		"ZED":      {Command: "zed"},
		"NEOVIM":   {Command: "nvim"},
	}

	result := map[string]any{
		"available": false,
		"path":      nil,
	}

	if info, ok := editors[strings.ToUpper(editorType)]; ok {
		if path, err := exec.LookPath(info.Command); err == nil {
			result["available"] = true
			result["path"] = path
		}
	}

	success(w, result)
}

// handleRemoteProjects handles GET /api/remote/projects.
func (h *Handler) handleRemoteProjects(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"projects": []any{},
	})
}

// handleOpenRemoteEditor handles POST /api/open-remote-editor/workspace.
func (h *Handler) handleOpenRemoteEditor(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"success":false,"error_data":{"message":"remote editor not available in local mode"},"message":"not implemented"}`, http.StatusNotImplemented)
}
