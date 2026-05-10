package handler

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/xuzhiping7/ai-kanban/internal/service"
)

const (
	// wsWriteWait is the time allowed to write a message to the peer.
	wsWriteWait = 10 * time.Second
	// wsPongWait is the time allowed to read the next pong message from the peer.
	wsPongWait = 60 * time.Second
	// wsPingPeriod sends pings to peer with this period. Must be less than wsPongWait.
	wsPingPeriod = (wsPongWait * 9) / 10
)

// upgrader is the shared WebSocket upgrader for all WS endpoints.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// wsStreamConfig configures a WebSocket stream session.
type wsStreamConfig struct {
	// InitialMessages are sent before the Ready signal.
	InitialMessages []service.LogMsg
	// Subscribe returns a channel of live messages and an unsubscribe function.
	Subscribe func() (<-chan service.LogMsg, func())
	// Convert converts a LogMsg to WS JSON bytes. Returns nil to skip.
	Convert func(service.LogMsg) ([]byte, error)
}

// handleWSStream upgrades HTTP to WebSocket and streams LogMsg values.
//
// Protocol:
// 1. Send initial snapshot patches
// 2. Send {"Ready":true}
// 3. Stream live messages from subscription
// 4. On disconnect, clean up
func handleWSStream(w http.ResponseWriter, r *http.Request, cfg wsStreamConfig) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	// Send initial snapshot messages.
	for _, msg := range cfg.InitialMessages {
		data, err := cfg.Convert(msg)
		if err != nil || data == nil {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}

	// Send Ready signal.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"Ready":true}`)); err != nil {
		return
	}

	// Subscribe to live updates.
	ch, unsubscribe := cfg.Subscribe()
	defer unsubscribe()

	// Coordinated shutdown.
	var once sync.Once
	done := make(chan struct{})
	closeDone := func() { once.Do(func() { close(done) }) }

	// Write mutex: gorilla/websocket allows one concurrent reader and one concurrent writer.
	// We serialize all writes (data + pings) through this mutex.
	var writeMu sync.Mutex

	// Writer goroutine: reads from channel, converts, writes to WS.
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for msg := range ch {
			data, err := cfg.Convert(msg)
			if err != nil {
				slog.Debug("ws convert error", "error", err)
				continue
			}
			if data == nil {
				continue
			}
			writeMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			err = conn.WriteMessage(websocket.TextMessage, data)
			writeMu.Unlock()
			if err != nil {
				closeDone()
				return
			}
			// If this is a finished message, close after sending.
			if msg.Kind == service.LogMsgFinished {
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
					closeDone()
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Reader: consume client messages (detect close frames).
	conn.SetReadLimit(512 * 1024)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			closeDone()
			return
		}
	}
}

// defaultWSConvert converts LogMsg using ToWSMessage (for event streams).
func defaultWSConvert(msg service.LogMsg) ([]byte, error) {
	return msg.ToWSMessage()
}

// logEntryWSConvert converts LogMsg using ToLogEntryWSMessage for stdout/stderr,
// and ToWSMessage for other kinds (for log streams).
func logEntryWSConvert(msg service.LogMsg) ([]byte, error) {
	if msg.Kind == service.LogMsgStdout || msg.Kind == service.LogMsgStderr {
		return msg.ToLogEntryWSMessage()
	}
	return msg.ToWSMessage()
}
