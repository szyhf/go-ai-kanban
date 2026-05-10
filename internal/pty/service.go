package pty

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty/v2"
)

// Session represents an active PTY terminal session.
type Session struct {
	ID        string
	Cmd       *exec.Cmd
	Ptmx      *os.File
	OutputCh  chan []byte
	DoneCh    chan struct{}
	closeOnce sync.Once
	logger    *slog.Logger
}

// Service manages PTY terminal sessions.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*Session
	logger   *slog.Logger
}

// NewService creates a new PTY service.
func NewService() *Service {
	return &Service{
		sessions: make(map[string]*Session),
		logger:   slog.Default(),
	}
}

// CreateSessionInput holds the parameters for creating a PTY session.
type CreateSessionInput struct {
	ID         string
	WorkingDir string
	Shell      string
	Cols       uint16
	Rows       uint16
}

// CreateSession starts a new shell process in a PTY and returns the session.
func (s *Service) CreateSession(input CreateSessionInput) (*Session, error) {
	shell := input.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell)
	cmd.Dir = input.WorkingDir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: input.Cols,
		Rows: input.Rows,
	})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	sess := &Session{
		ID:       input.ID,
		Cmd:      cmd,
		Ptmx:     ptmx,
		OutputCh: make(chan []byte, 1024),
		DoneCh:   make(chan struct{}),
		logger:   s.logger,
	}

	// Read PTY output in background.
	go sess.readOutput()

	// Wait for process exit in background.
	go sess.waitProcess()

	s.mu.Lock()
	s.sessions[input.ID] = sess
	s.mu.Unlock()

	s.logger.Info("PTY 会话已创建", "id", input.ID, "pid", cmd.Process.Pid)
	return sess, nil
}

// GetSession returns a session by ID.
func (s *Service) GetSession(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

// CloseSession closes and removes a PTY session.
func (s *Service) CloseSession(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	sess.Close()
	s.logger.Info("PTY 会话已关闭", "id", id)
	return nil
}

// CloseAll closes all active PTY sessions.
func (s *Service) CloseAll() {
	s.mu.Lock()
	sessions := make(map[string]*Session)
	for k, v := range s.sessions {
		sessions[k] = v
	}
	s.sessions = make(map[string]*Session)
	s.mu.Unlock()

	for _, sess := range sessions {
		sess.Close()
	}
}

// ListSessions returns all active session IDs.
func (s *Service) ListSessions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Write writes input data to the PTY.
func (sess *Session) Write(data []byte) error {
	_, err := sess.Ptmx.Write(data)
	return err
}

// Resize changes the PTY window size.
func (sess *Session) Resize(cols, rows uint16) error {
	return pty.Setsize(sess.Ptmx, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
}

// Close shuts down the PTY session.
func (sess *Session) Close() {
	sess.closeOnce.Do(func() {
		// Kill the process group.
		if sess.Cmd.Process != nil {
			_ = syscall.Kill(-sess.Cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = sess.Ptmx.Close()
		close(sess.OutputCh)
		close(sess.DoneCh)
	})
}

// readOutput reads from the PTY master and sends data to the output channel.
func (sess *Session) readOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := sess.Ptmx.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case sess.OutputCh <- data:
			default:
				// Drop if channel is full.
			}
		}
		if err != nil {
			return
		}
	}
}

// waitProcess waits for the shell process to exit.
func (sess *Session) waitProcess() {
	_ = sess.Cmd.Wait()
	sess.Close()
}
