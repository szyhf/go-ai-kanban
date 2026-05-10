package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// handleDiffStreamWS handles GET /api/workspaces/{id}/git/diff/ws?stats_only=false
func (h *Handler) handleDiffStreamWS(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	workspace, err := h.wsRepo.FindByID(id)
	if err != nil || workspace == nil {
		notFound(w, "workspace not found")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket 升级失败", "error", err)
		return
	}
	defer conn.Close()

	// Load workspace repos.
	wsRepos, err := h.wsRepoRepo.FindByWorkspaceIDWithRepos(id)
	if err != nil {
		slog.Error("加载 workspace repos", "error", err)
		return
	}

	// Write mutex for concurrent writes.
	var writeMu sync.Mutex

	// Compute and send initial diff snapshot.
	statsOnly := r.URL.Query().Get("stats_only") == "true"
	diffs := h.computeWorkspaceDiffs(workspace, wsRepos, statsOnly)
	writeMu.Lock()
	err = sendDiffSnapshot(conn, diffs)
	writeMu.Unlock()
	if err != nil {
		return
	}

	// Send Ready.
	writeMu.Lock()
	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"Ready":true}`))
	writeMu.Unlock()
	if err != nil {
		return
	}

	// Coordinated shutdown.
	done := make(chan struct{})

	// Poll for diff changes every 5 seconds.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var prevDiffs []service.LogMsg
		for {
			select {
			case <-ticker.C:
				newDiffs := h.computeWorkspaceDiffs(workspace, wsRepos, statsOnly)
				if !diffsEqual(prevDiffs, newDiffs) {
					writeMu.Lock()
					sendErr := sendDiffSnapshot(conn, newDiffs)
					writeMu.Unlock()
					if sendErr != nil {
						return
					}
					prevDiffs = newDiffs
				}
			case <-done:
				return
			}
		}
	}()

	// Ping goroutine.
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait))
				writeMu.Unlock()
				if err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Read client messages until disconnect.
	conn.SetReadLimit(512 * 1024)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			close(done)
			return
		}
	}
}

// computeWorkspaceDiffs computes diffs for all repos in a workspace.
func (h *Handler) computeWorkspaceDiffs(ws *domain.Workspace, wsRepos []domain.RepoWithTargetBranch, statsOnly bool) []service.LogMsg {
	var allDiffs []any
	for _, wr := range wsRepos {
		diffs, err := h.gitSvc.GetDiffs(wr.Path, nil, nil, wr.ID.String())
		if err != nil {
			slog.Debug("计算 diff", "repo", wr.Path, "error", err)
			continue
		}
		if statsOnly {
			for i := range diffs {
				diffs[i].OldContent = ""
				diffs[i].NewContent = ""
				diffs[i].ContentOmitted = true
			}
		}
		for _, d := range diffs {
			allDiffs = append(allDiffs, d)
		}
	}

	if allDiffs == nil {
		allDiffs = []any{}
	}

	return []service.LogMsg{
		service.NewPatchLogMsg(service.PatchOperation{
			Op:    "replace",
			Path:  "/diffs",
			Value: allDiffs,
		}),
	}
}

// sendDiffSnapshot sends diff messages as JsonPatch messages.
func sendDiffSnapshot(conn *websocket.Conn, msgs []service.LogMsg) error {
	for _, msg := range msgs {
		data, err := msg.ToWSMessage()
		if err != nil || data == nil {
			continue
		}
		conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
	}
	return nil
}

// diffsEqual compares two diff message slices.
func diffsEqual(a, b []service.LogMsg) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		aj, err := json.Marshal(a[i])
		if err != nil {
			return false
		}
		bj, err := json.Marshal(b[i])
		if err != nil {
			return false
		}
		if string(aj) != string(bj) {
			return false
		}
	}
	return true
}
