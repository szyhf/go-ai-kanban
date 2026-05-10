package handler

import (
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// handleRawLogsWS handles GET /api/execution-processes/{id}/raw-logs/ws
func (h *Handler) handleRawLogsWS(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	// Get the per-execution MsgStore.
	var execMsgStore *service.MsgStore
	if h.containerSvc != nil {
		execMsgStore = h.containerSvc.GetMsgStore(id)
	}

	// If no MsgStore, send finished and close.
	if execMsgStore == nil {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.WriteMessage(websocket.TextMessage, []byte(`{"finished":true}`))
		return
	}

	// Build initial messages from history.
	history := execMsgStore.History()
	var initial []service.LogMsg
	for _, msg := range history {
		if msg.Kind == service.LogMsgStdout || msg.Kind == service.LogMsgStderr {
			initial = append(initial, msg)
		}
	}

	cfg := wsStreamConfig{
		InitialMessages: initial,
		Subscribe: func() (<-chan service.LogMsg, func()) {
			return execMsgStore.Subscribe()
		},
		Convert: logEntryWSConvert,
	}
	handleWSStream(w, r, cfg)
}
