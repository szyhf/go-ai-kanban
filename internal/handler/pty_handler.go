package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/xuzhiping7/ai-kanban/internal/pty"
)

const (
	// wsWriteWait is the time allowed to write a message to the peer.
	wsWriteWait = 10 * time.Second
	// wsPongWait is the time allowed to read the next pong message from the peer.
	wsPongWait = 60 * time.Second
	// wsPingPeriod sends pings to peer with this period. Must be less than wsPongWait.
	wsPingPeriod = (wsPongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ptyHandler handles WebSocket connections for PTY terminal sessions.
type ptyHandler struct {
	ptySvc  *pty.Service
	logger  *slog.Logger
}

// newPTYHandler creates a new PTY handler.
func newPTYHandler(ptySvc *pty.Service) *ptyHandler {
	return &ptyHandler{
		ptySvc: ptySvc,
		logger: slog.Default(),
	}
}

// handleTerminal handles the WebSocket terminal endpoint.
// Query params: workspace_id (optional), cols (default 80), rows (default 24).
func (h *ptyHandler) handleTerminal(w http.ResponseWriter, r *http.Request) {
	cols := uint16(80)
	rows := uint16(24)

	if v := r.URL.Query().Get("cols"); v != "" {
		if parsed, err := parseUint16(v); err == nil && parsed > 0 {
			cols = parsed
		}
	}
	if v := r.URL.Query().Get("rows"); v != "" {
		if parsed, err := parseUint16(v); err == nil && parsed > 0 {
			rows = parsed
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	sessionID := generateSessionID()
	workingDir := r.URL.Query().Get("working_dir")

	sess, err := h.ptySvc.CreateSession(pty.CreateSessionInput{
		ID:         sessionID,
		WorkingDir: workingDir,
		Cols:       cols,
		Rows:       rows,
	})
	if err != nil {
		h.logger.Error("failed to create pty session", "error", err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer h.ptySvc.CloseSession(sessionID)

	// PTY output → WebSocket.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for data := range sess.OutputCh {
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return
			}
		}
	}()

	// WebSocket → PTY input.
	conn.SetReadLimit(512 * 1024)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Ping goroutine.
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Check for resize messages (JSON text messages).
		if msgType == websocket.TextMessage {
			var resizeMsg struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if err := json.Unmarshal(data, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
				_ = sess.Resize(resizeMsg.Cols, resizeMsg.Rows)
				continue
			}
		}

		// Binary or text data is written to PTY as input.
		if err := sess.Write(data); err != nil {
			break
		}
	}
}

// generateSessionID creates a simple unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("pty-%d", time.Now().UnixNano())
}

// parseUint16 parses a string as uint16.
func parseUint16(s string) (uint16, error) {
	var v uint16
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid digit: %c", c)
		}
		v = v*10 + uint16(c-'0')
	}
	return v, nil
}
