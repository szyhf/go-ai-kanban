package handler

import (
	"net/http"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// handleSearch handles GET /api/search.
func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := getQuery(r, "q")
	if query == "" {
		badRequest(w, "q is required")
		return
	}

	// Collect repo IDs to search in.
	repoIDStrs := getQuerySlice(r, "repo_ids[]")
	var repos []domain.Repo

	if len(repoIDStrs) > 0 {
		for _, s := range repoIDStrs {
			id, err := domain.ParseUUID(s)
			if err != nil {
				continue
			}
			repo, err := h.repoSvc.FindByID(id)
			if err != nil || repo == nil {
				continue
			}
			repos = append(repos, *repo)
		}
	} else {
		// Search all repos.
		allRepos, err := h.repoSvc.FindAll()
		if err != nil {
			internalError(w, "failed to list repos: "+err.Error())
			return
		}
		repos = allRepos
	}

	results, err := h.repoSvc.SearchFiles(repos, query)
	if err != nil {
		internalError(w, "search failed: "+err.Error())
		return
	}
	success(w, results)
}
