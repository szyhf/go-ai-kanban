package executor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// ProtocolPeer manages bidirectional JSON-over-stdio communication with Claude Code CLI.
// It reads stdout line-by-line, dispatches control requests, and sends responses.
type ProtocolPeer struct {
	stdin  io.Writer
	mu     sync.Mutex
	logger *slog.Logger
}

// NewProtocolPeer creates a new protocol peer and starts the read loop.
// The read loop runs in a goroutine and dispatches messages to the handler.
// It signals completion via the done channel when a "result" message is received.
func NewProtocolPeer(stdin io.Writer, stdout io.Reader, handler *ClaudeAgentClient, cancelCh <-chan struct{}, done chan<- ExitResult, rawLines chan<- string) *ProtocolPeer {
	p := &ProtocolPeer{
		stdin:  stdin,
		logger: slog.Default(),
	}
	go p.readLoop(stdout, handler, cancelCh, done, rawLines)
	return p
}

// readLoop reads stdout line-by-line, parses JSON messages, and dispatches.
func (p *ProtocolPeer) readLoop(stdout io.Reader, handler *ClaudeAgentClient, cancelCh <-chan struct{}, done chan<- ExitResult, rawLines chan<- string) {
	scanner := bufio.NewScanner(stdout)
	// Allow up to 10MB lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Forward raw line to logging channel.
		if rawLines != nil {
			select {
			case rawLines <- line:
			default:
				// Drop if channel is full.
			}
		}

		var msg CLIMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Not JSON — likely stderr bleed-through or non-structured output.
			p.logger.Debug("non-JSON stdout line", "line", truncate(line, 200))
			continue
		}

		switch msg.Type {
		case "control_request":
			p.handleControlRequest(msg, handler)
		case "result":
			p.logger.Info("claude code result received")
			select {
			case done <- ExitResult{Code: 0}:
			default:
			}
			return
		default:
			// System, assistant, user, etc. — log and continue.
			p.logger.Debug("claude message", "type", msg.Type)
		}
	}

	if err := scanner.Err(); err != nil {
		p.logger.Error("stdout scanner error", "error", err)
	}
}

// handleControlRequest dispatches a control request to the handler.
func (p *ProtocolPeer) handleControlRequest(msg CLIMessage, handler *ClaudeAgentClient) {
	var req ControlRequest
	if err := json.Unmarshal(msg.Request, &req); err != nil {
		p.logger.Error("failed to parse control request", "error", err)
		return
	}

	p.logger.Debug("control request", "subtype", req.Subtype, "tool", req.ToolName)

	switch req.Subtype {
	case "can_use_tool":
		result, err := handler.OnCanUseTool(req.ToolName, req.Input, req.ToolUseID)
		if err != nil {
			p.logger.Error("can_use_tool handler error", "error", err)
			return
		}
		resp := ControlResponse{
			Type:     "control_response",
			Accepted: !result.Deny,
		}
		if result.Deny {
			resp.Response = PermissionResponse{
				Behavior: "deny",
			}
		} else {
			resp.Response = PermissionResponse{
				Behavior: "allow",
			}
		}
		p.sendControlResponse(msg.RequestID, resp)
	case "hook_callback":
		p.handleHookCallback(msg.RequestID, req, handler)
	default:
		p.logger.Warn("unknown control request subtype", "subtype", req.Subtype)
	}
}

// handleHookCallback handles hook callback requests.
func (p *ProtocolPeer) handleHookCallback(requestID string, req ControlRequest, handler *ClaudeAgentClient) {
	switch req.ToolName {
	case CallbackAutoApprove:
		// Auto-approve in plan mode.
		resp := ControlResponse{
			Type:     "control_response",
			Accepted: true,
		}
		p.sendControlResponse(requestID, resp)
	default:
		// Forward to handler.
		result, err := handler.OnHookCallback(req.ToolName, req.Input)
		if err != nil {
			p.logger.Error("hook callback error", "error", err)
			return
		}
		resp := ControlResponse{
			Type:     "control_response",
			Accepted: !result.Deny,
		}
		p.sendControlResponse(requestID, resp)
	}
}

// SendUserMessage sends a user prompt to Claude Code via stdin.
func (p *ProtocolPeer) SendUserMessage(content string) error {
	msg := UserMessage{
		Type: "user",
		Message: MessageBody{
			Role:    "user",
			Content: content,
		},
	}
	return p.sendJSON(msg)
}

// Initialize sends the initialize control request with hooks configuration.
func (p *ProtocolPeer) Initialize(hooks json.RawMessage) error {
	req := SDKControlRequest{
		Type:      "control_request",
		RequestID: "init",
		Request:   hooks,
	}
	return p.sendJSON(req)
}

// SetPermissionMode changes the permission mode.
func (p *ProtocolPeer) SetPermissionMode(mode PermissionMode) error {
	payload, _ := json.Marshal(SetPermissionModeRequest{
		Subtype: "set_permission_mode",
		Mode:    mode,
	})
	req := SDKControlRequest{
		Type:      "control_request",
		RequestID: "perm_mode",
		Request:   payload,
	}
	return p.sendJSON(req)
}

// Interrupt sends an interrupt signal to Claude Code.
func (p *ProtocolPeer) Interrupt() error {
	payload, _ := json.Marshal(InterruptRequest{Subtype: "interrupt"})
	req := SDKControlRequest{
		Type:      "control_request",
		RequestID: "interrupt",
		Request:   payload,
	}
	return p.sendJSON(req)
}

// sendControlResponse sends a response to a control request.
func (p *ProtocolPeer) sendControlResponse(requestID string, resp ControlResponse) {
	resp.Type = "control_response"
	wrapper := struct {
		Type       string           `json:"type"`
		RequestID  string           `json:"request_id"`
		Accepted   bool             `json:"accepted"`
		Response   PermissionResponse `json:"response,omitempty"`
	}{
		Type:      "control_response",
		RequestID: requestID,
		Accepted:  resp.Accepted,
		Response:  resp.Response,
	}
	if err := p.sendJSON(wrapper); err != nil {
		p.logger.Error("failed to send control response", "error", err)
	}
}

// sendJSON writes a JSON message to stdin, protected by a mutex.
func (p *ProtocolPeer) sendJSON(v interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')
	_, err = p.stdin.Write(data)
	return err
}

// truncate shortens a string for logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
