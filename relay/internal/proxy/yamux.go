package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/hashicorp/yamux"
)

// YamuxSession wraps a yamux.Session for multiplexing streams over a single
// connection. It allows proxying multiple HTTP requests concurrently over
// one WebSocket connection.
type YamuxSession struct {
	session *yamux.Session
	mu      sync.Mutex
}

// NewYamuxSession creates a new Yamux session over an existing connection.
// isClient should be true if this is the client side (initiates streams).
func NewYamuxSession(conn net.Conn, isClient bool) (*YamuxSession, error) {
	var session *yamux.Session
	var err error
	if isClient {
		session, err = yamux.Client(conn, nil)
	} else {
		session, err = yamux.Server(conn, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("create yamux session: %w", err)
	}
	return &YamuxSession{session: session}, nil
}

// OpenStream opens a new bidirectional stream.
func (y *YamuxSession) OpenStream() (net.Conn, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.session.Open()
}

// AcceptStream accepts an incoming stream.
func (y *YamuxSession) AcceptStream() (net.Conn, error) {
	return y.session.Accept()
}

// Close shuts down the yamux session.
func (y *YamuxSession) Close() error {
	return y.session.Close()
}

// IsClosed returns whether the session is closed.
func (y *YamuxSession) IsClosed() bool {
	return y.session.IsClosed()
}

// ProxyRequest represents an HTTP request to be sent over a yamux stream.
type ProxyRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// ProxyResponse represents an HTTP response received over a yamux stream.
type ProxyResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// WriteRequest writes a proxy request as JSON to the stream.
func WriteRequest(w io.Writer, req *ProxyRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal proxy request: %w", err)
	}
	// Write length-prefixed JSON.
	lenBuf := make([]byte, 4)
	lenBuf[0] = byte(len(data) >> 24)
	lenBuf[1] = byte(len(data) >> 16)
	lenBuf[2] = byte(len(data) >> 8)
	lenBuf[3] = byte(len(data))
	if _, err := w.Write(append(lenBuf, data...)); err != nil {
		return fmt.Errorf("write proxy request: %w", err)
	}
	return nil
}

// ReadRequest reads a proxy request from the stream.
func ReadRequest(r io.Reader) (*ProxyRequest, error) {
	return readLengthPrefixed[ProxyRequest](r)
}

// WriteResponse writes a proxy response as JSON to the stream.
func WriteResponse(w io.Writer, resp *ProxyResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal proxy response: %w", err)
	}
	lenBuf := make([]byte, 4)
	lenBuf[0] = byte(len(data) >> 24)
	lenBuf[1] = byte(len(data) >> 16)
	lenBuf[2] = byte(len(data) >> 8)
	lenBuf[3] = byte(len(data))
	if _, err := w.Write(append(lenBuf, data...)); err != nil {
		return fmt.Errorf("write proxy response: %w", err)
	}
	return nil
}

// ReadResponse reads a proxy response from the stream.
func ReadResponse(r io.Reader) (*ProxyResponse, error) {
	resp, err := readLengthPrefixed[ProxyResponse](r)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// readLengthPrefixed reads a 4-byte length prefix followed by JSON data.
func readLengthPrefixed[T any](r io.Reader) (*T, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read length prefix: %w", err)
	}
	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	if length > 10*1024*1024 { // 10MB max
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read message body: %w", err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}
	return &result, nil
}
