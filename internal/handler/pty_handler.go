package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/xuzhiping7/ai-kanban/internal/pty"
)

// ptyHandler handles WebSocket connections for PTY terminal sessions.
type ptyHandler struct {
	ptySvc *pty.Service
	logger *slog.Logger
}

// newPTYHandler creates a new PTY handler.
func newPTYHandler(ptySvc *pty.Service) *ptyHandler {
	return &ptyHandler{
		ptySvc: ptySvc,
		logger: slog.Default(),
	}
}

// ptyInputMsg represents an input message from the frontend.
type ptyInputMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"` // base64 encoded
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// ptyOutputMsg represents an output message to the frontend.
type ptyOutputMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"` // base64 encoded
}

// handleTerminal handles the WebSocket terminal endpoint.
// Frontend protocol (JSON text messages):
//
//	Input:  {"type":"input","data":"<base64>"}
//	Resize: {"type":"resize","cols":N,"rows":N}
//	Output: {"type":"output","data":"<base64>"}
//	Exit:   {"type":"exit"}
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
		h.logger.Error("WebSocket 升级失败", "error", err)
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
		h.logger.Error("创建 PTY 会话失败", "error", err)
		errMsg, _ := json.Marshal(ptyOutputMsg{Type: "exit"})
		_ = conn.WriteMessage(websocket.TextMessage, errMsg)
		return
	}
	defer h.ptySvc.CloseSession(sessionID)

	// Write mutex: gorilla/websocket allows one concurrent writer.
	var writeMu sync.Mutex

	// PTY output → WebSocket (JSON text with base64 data).
	done := make(chan struct{})
	go func() {
		defer close(done)
		for data := range sess.OutputCh {
			encoded := base64.StdEncoding.EncodeToString(data)
			msg, err := json.Marshal(ptyOutputMsg{Type: "output", Data: encoded})
			if err != nil {
				return
			}
			writeMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			err = conn.WriteMessage(websocket.TextMessage, msg)
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
		// Send exit message when PTY output channel closes.
		exitMsg, _ := json.Marshal(ptyOutputMsg{Type: "exit"})
		writeMu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, exitMsg)
		writeMu.Unlock()
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

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Parse JSON text messages.
		if msgType == websocket.TextMessage {
			var msg ptyInputMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case "input":
				decoded, err := base64.StdEncoding.DecodeString(msg.Data)
				if err != nil {
					continue
				}
				if err := sess.Write(decoded); err != nil {
					return
				}
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = sess.Resize(msg.Cols, msg.Rows)
				}
			}
		}
	}
}

// generateSessionID creates a simple unique session ID.
func generateSessionID() string {
	return formatPtyID(time.Now().UnixNano())
}

// formatPtyID formats a PTY session ID.
func formatPtyID(nano int64) string {
	return formatUint(int64(nano), "pty-")
}

// formatUint formats an int64 with a prefix.
func formatUint(v int64, prefix string) string {
	if v == 0 {
		return prefix + "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return prefix + string(buf[i:])
}

// parseUint16 parses a string as uint16.
func parseUint16(s string) (uint16, error) {
	var v uint16
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errInvalidDigit(c)
		}
		v = v*10 + uint16(c-'0')
	}
	return v, nil
}

// errInvalidDigit returns an error for an invalid digit.
func errInvalidDigit(c rune) error {
	return fmt.Errorf("invalid digit: %c", c)
}
