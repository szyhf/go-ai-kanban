package service

import (
	"encoding/json"
	"fmt"
	"sync"
)

// PatchOperation represents a JSON Patch operation (RFC 6902).
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// LogMsg represents a message in the log message store.
type LogMsg struct {
	// Kind identifies the message type: "stdout", "stderr", "patch", "ready", "finished", "session_id", "message_id".
	Kind string `json:"kind"`
	// Data holds the message payload.
	Data string `json:"data,omitempty"`
	// Patch holds a JSON Patch operation (when Kind == "patch").
	Patch *PatchOperation `json:"patch,omitempty"`
}

// LogMsg kinds.
const (
	LogMsgStdout    = "stdout"
	LogMsgStderr    = "stderr"
	LogMsgPatch     = "patch"
	LogMsgReady     = "ready"
	LogMsgFinished  = "finished"
	LogMsgSessionID = "session_id"
	LogMsgMessageID = "message_id"
)

// NewStdoutLogMsg creates a stdout log message.
func NewStdoutLogMsg(s string) LogMsg {
	return LogMsg{Kind: LogMsgStdout, Data: s}
}

// NewStderrLogMsg creates a stderr log message.
func NewStderrLogMsg(s string) LogMsg {
	return LogMsg{Kind: LogMsgStderr, Data: s}
}

// NewPatchLogMsg creates a JSON Patch log message.
func NewPatchLogMsg(op PatchOperation) LogMsg {
	return LogMsg{Kind: LogMsgPatch, Patch: &op}
}

// NewReadyLogMsg creates a ready signal message.
func NewReadyLogMsg() LogMsg {
	return LogMsg{Kind: LogMsgReady}
}

// NewFinishedLogMsg creates a finished signal message.
func NewFinishedLogMsg() LogMsg {
	return LogMsg{Kind: LogMsgFinished}
}

// ToSSEEvent converts the log message to an SSE-compatible string.
func (m LogMsg) ToSSEEvent() (event string, data string) {
	switch m.Kind {
	case LogMsgPatch:
		b, _ := json.Marshal(m.Patch)
		return "patch", string(b)
	case LogMsgReady:
		return "ready", `{"ready":true}`
	case LogMsgFinished:
		return "finished", `{"finished":true}`
	default:
		return m.Kind, m.Data
	}
}

// wsPatchMsg is the WS JSON format for JsonPatch messages.
type wsPatchMsg struct {
	JsonPatch []*PatchOperation `json:"JsonPatch"`
}

// LogEntryValue represents a log entry in the format expected by the frontend PatchType.
type LogEntryValue struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// ToWSMessage converts the LogMsg to the frontend WebSocket JSON format.
// Returns nil, nil for kinds that should be skipped (stdout/stderr).
func (m LogMsg) ToWSMessage() ([]byte, error) {
	switch m.Kind {
	case LogMsgReady:
		return []byte(`{"Ready":true}`), nil
	case LogMsgFinished:
		return []byte(`{"finished":true}`), nil
	case LogMsgPatch:
		return json.Marshal(wsPatchMsg{JsonPatch: []*PatchOperation{m.Patch}})
	default:
		return nil, nil
	}
}

// ToLogEntryWSMessage converts stdout/stderr LogMsg to a log entry JsonPatch message.
// Returns nil, nil for non-log kinds.
func (m LogMsg) ToLogEntryWSMessage() ([]byte, error) {
	entryType := ""
	switch m.Kind {
	case LogMsgStdout:
		entryType = "STDOUT"
	case LogMsgStderr:
		entryType = "STDERR"
	default:
		return nil, nil
	}
	op := &PatchOperation{
		Op:    "add",
		Path:  "/entries/-",
		Value: LogEntryValue{Type: entryType, Content: m.Data},
	}
	return json.Marshal(wsPatchMsg{JsonPatch: []*PatchOperation{op}})
}

// ApproxBytes estimates the memory usage of this message.
func (m LogMsg) ApproxBytes() int {
	n := len(m.Kind) + len(m.Data)
	if m.Patch != nil {
		b, _ := json.Marshal(m.Patch)
		n += len(b)
	}
	return n
}

const (
	// historyBytesLimit is the maximum total bytes stored in history (100 MB).
	historyBytesLimit = 100 * 1024 * 1024
	// subscriberBuffer is the buffered channel capacity for each subscriber.
	subscriberBuffer = 10000
)

// storedMsg wraps a LogMsg with its byte size.
type storedMsg struct {
	msg   LogMsg
	bytes int
}

// MsgStore is a broadcast message store with bounded history.
// It supports multiple subscribers that receive all pushed messages.
type MsgStore struct {
	mu          sync.RWMutex
	history     []storedMsg
	totalBytes  int
	subscribers map[int64]chan LogMsg
	nextID      int64
}

// NewMsgStore creates a new MsgStore.
func NewMsgStore() *MsgStore {
	return &MsgStore{
		subscribers: make(map[int64]chan LogMsg),
	}
}

// Push sends a message to all subscribers and appends it to history.
func (s *MsgStore) Push(msg LogMsg) {
	bytes := msg.ApproxBytes()

	s.mu.Lock()
	// Append to history, evicting oldest if over budget.
	s.history = append(s.history, storedMsg{msg: msg, bytes: bytes})
	s.totalBytes += bytes
	for s.totalBytes > historyBytesLimit && len(s.history) > 1 {
		evicted := s.history[0]
		s.history = s.history[1:]
		s.totalBytes -= evicted.bytes
	}

	// Send to all subscriber channels (non-blocking).
	for id, ch := range s.subscribers {
		select {
		case ch <- msg:
		default:
			// Subscriber is too slow; drop the message.
			// Broadcast behavior: lagged receivers miss messages.
			_ = id
		}
	}
	s.mu.Unlock()
}

// PushPatch is a convenience method to push a JSON Patch operation.
func (s *MsgStore) PushPatch(op PatchOperation) {
	s.Push(NewPatchLogMsg(op))
}

// Subscribe returns a channel that receives all future messages.
// The caller must consume from the channel to avoid blocking.
func (s *MsgStore) Subscribe() (<-chan LogMsg, func()) {
	ch := make(chan LogMsg, subscriberBuffer)

	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subscribers[id] = ch
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		close(ch)
		s.mu.Unlock()
	}

	return ch, unsubscribe
}

// History returns a copy of all stored messages.
func (s *MsgStore) History() []LogMsg {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := make([]LogMsg, len(s.history))
	for i, sm := range s.history {
		msgs[i] = sm.msg
	}
	return msgs
}

// SubscriberCount returns the number of active subscribers.
func (s *MsgStore) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// String implements fmt.Stringer.
func (s *MsgStore) String() string {
	return fmt.Sprintf("MsgStore{subscribers: %d, history: %d msgs, %d bytes}",
		s.SubscriberCount(), len(s.history), s.totalBytes)
}
