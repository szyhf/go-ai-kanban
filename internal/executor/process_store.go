package executor

import (
	"context"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// ProcessEntry holds the runtime state of an active execution process.
type ProcessEntry struct {
	// Cmd is the underlying OS process handle.
	Cmd *exec.Cmd
	// Cancel triggers graceful shutdown of the process.
	Cancel context.CancelFunc
	// MsgStore is the message store for streaming logs to subscribers.
	MsgStore *service.MsgStore
	// Done receives the exit result when the process completes.
	Done <-chan ExitResult
}

// ProcessStore is a thread-safe registry of active execution processes.
type ProcessStore struct {
	mu      sync.Mutex
	entries map[domain.UUID]*ProcessEntry
	logger  *slog.Logger
}

// NewProcessStore creates a new ProcessStore.
func NewProcessStore() *ProcessStore {
	return &ProcessStore{
		entries: make(map[domain.UUID]*ProcessEntry),
		logger:  slog.Default(),
	}
}

// Add registers a new active process.
func (s *ProcessStore) Add(id domain.UUID, entry *ProcessEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[id] = entry
	s.logger.Debug("process registered", "id", id)
}

// Get retrieves a process entry by ID. Returns nil if not found.
func (s *ProcessStore) Get(id domain.UUID) *ProcessEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entries[id]
}

// Remove removes a process entry from the store and returns it.
func (s *ProcessStore) Remove(id domain.UUID) *ProcessEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[id]
	delete(s.entries, id)
	return entry
}

// KillAll forcefully kills all active processes and clears the store.
func (s *ProcessStore) KillAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.entries {
		if err := KillProcessGroup(entry.Cmd); err != nil {
			s.logger.Warn("failed to kill process group", "id", id, "error", err)
		}
		if entry.Cancel != nil {
			entry.Cancel()
		}
		delete(s.entries, id)
	}
}

// List returns all active process IDs.
func (s *ProcessStore) List() []domain.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]domain.UUID, 0, len(s.entries))
	for id := range s.entries {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active processes.
func (s *ProcessStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
