package signaling

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
)

// Peer represents a single WebSocket connection managed by the Hub.
type Peer struct {
	ID       string // client ID or host ID
	Role     string // "client" or "host"
	ClientID string // for hosts: the owning client's ID; for clients: same as ID

	conn   *websocket.Conn
	mu     sync.Mutex // protects writes to conn
	seq    int64      // outgoing sequence counter
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPeer wraps a WebSocket connection into a managed Peer.
func NewPeer(conn *websocket.Conn) *Peer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Peer{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Send marshals and sends an Envelope to the peer.
func (p *Peer) Send(env *Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conn.Write(p.ctx, websocket.MessageText, data)
}

// SendPayload sends a typed message as a JSON envelope.
func (p *Peer) SendPayload(msgType MessageType, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	p.seq++
	return p.Send(&Envelope{
		Type:    msgType,
		Payload: raw,
		Seq:     p.seq,
	})
}

// SendError sends an error envelope to the peer.
func (p *Peer) SendError(code, message string) error {
	return p.SendPayload(TypeError, ErrorPayload{Code: code, Message: message})
}

// ReadMessage reads the next message from the WebSocket connection.
func (p *Peer) ReadMessage() ([]byte, error) {
	_, data, err := p.conn.Read(p.ctx)
	return data, err
}

// Close gracefully closes the WebSocket connection.
func (p *Peer) Close() error {
	p.cancel()
	return p.conn.Close(websocket.StatusNormalClosure, "peer closed")
}

// Context returns the peer's context (cancelled on close).
func (p *Peer) Context() context.Context {
	return p.ctx
}
