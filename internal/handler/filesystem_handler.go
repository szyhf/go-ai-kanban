package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// registerFilesystemRoutes registers filesystem-related routes.
func (h *Handler) registerFilesystemRoutes(r chi.Router) {
	r.Get("/directory", h.listDirectory)
	r.Get("/git-repos", h.listGitRepos)
}

// listDirectory handles GET /api/filesystem/directory.
func (h *Handler) listDirectory(w http.ResponseWriter, r *http.Request) {
	path := getQuery(r, "path")

	resp, err := h.filesystem.ListDirectory(path)
	if err != nil {
		internalError(w, "failed to list directory: "+err.Error())
		return
	}
	success(w, resp)
}

// listGitRepos handles GET /api/filesystem/git-repos.
func (h *Handler) listGitRepos(w http.ResponseWriter, r *http.Request) {
	path := getQuery(r, "path")
	maxDepth := 5

	if d := getQuery(r, "max_depth"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			maxDepth = v
		}
	}

	var entries interface{}
	var err error

	if path != "" {
		entries, err = h.filesystem.ListGitRepos(path, maxDepth)
	} else {
		entries, err = h.filesystem.ListCommonGitRepos(maxDepth)
	}

	if err != nil {
		internalError(w, "failed to find git repos: "+err.Error())
		return
	}
	success(w, entries)
}
