package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleListReleases handles GET /api/releases.
// Fetches releases from GitHub API with a timeout.
func (h *Handler) handleListReleases(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get("https://api.github.com/repos/xuzhiping7/ai-kanban/releases?per_page=5")
	if err != nil {
		// Return empty on network error — not critical for local mode.
		success(w, map[string]any{"releases": []any{}})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		success(w, map[string]any{"releases": []any{}})
		return
	}

	var releases []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		success(w, map[string]any{"releases": []any{}})
		return
	}

	// Filter to only include fields the frontend needs
	type releaseInfo struct {
		ID          int    `json:"id"`
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Prerelease  bool   `json:"prerelease"`
		Draft       bool   `json:"draft"`
	}

	var result []releaseInfo
	for _, rel := range releases {
		info := releaseInfo{}
		if v, ok := rel["id"]; ok {
			info.ID = int(v.(float64))
		}
		if v, ok := rel["tag_name"]; ok {
			info.TagName = v.(string)
		}
		if v, ok := rel["name"]; ok {
			info.Name = v.(string)
		}
		if v, ok := rel["body"]; ok {
			info.Body = v.(string)
		}
		if v, ok := rel["html_url"]; ok {
			info.HTMLURL = v.(string)
		}
		if v, ok := rel["published_at"]; ok {
			info.PublishedAt = v.(string)
		}
		if v, ok := rel["prerelease"]; ok {
			info.Prerelease = v.(bool)
		}
		if v, ok := rel["draft"]; ok {
			info.Draft = v.(bool)
		}
		result = append(result, info)
	}

	if result == nil {
		result = []releaseInfo{}
	}

	success(w, map[string]any{"releases": result})
}
