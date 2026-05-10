package handler

import (
	"net/http"
)

// handleListReleases handles GET /api/releases.
func (h *Handler) handleListReleases(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{
		"releases": []interface{}{},
	})
}
