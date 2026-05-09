package handler

import (
	"fmt"
	"net/http"

	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// handleSSE handles GET /api/events — Server-Sent Events stream.
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send history first.
	for _, msg := range h.msgStore.History() {
		event, data := msg.ToSSEEvent()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	}
	flusher.Flush()

	// Subscribe to live events.
	ch, unsub := h.msgStore.Subscribe()
	defer unsub()

	// Send ready signal.
	readyMsg := service.NewReadyLogMsg()
	if event, data := readyMsg.ToSSEEvent(); event != "" {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			event, data := msg.ToSSEEvent()
			if event == "" {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flusher.Flush()
		}
	}
}
