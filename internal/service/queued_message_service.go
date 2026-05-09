package service

import (
	"sync"
	"time"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

// QueuedMessage represents a message queued for a session.
type QueuedMessage struct {
	SessionID domain.UUID
	Data      domain.DraftFollowUpData
	QueuedAt  time.Time
}

// QueueStatus represents the status of a session's message queue.
type QueueStatus struct {
	IsEmpty bool
	Msg     *QueuedMessage
}

// QueuedMessageService manages per-session queued messages in memory.
type QueuedMessageService struct {
	queue sync.Map // domain.UUID → QueuedMessage
}

// NewQueuedMessageService creates a new QueuedMessageService.
func NewQueuedMessageService() *QueuedMessageService {
	return &QueuedMessageService{}
}

// QueueMessage queues a message for a session, replacing any existing one.
func (s *QueuedMessageService) QueueMessage(sessionID domain.UUID, data domain.DraftFollowUpData) QueuedMessage {
	msg := QueuedMessage{
		SessionID: sessionID,
		Data:      data,
		QueuedAt:  time.Now(),
	}
	s.queue.Store(sessionID, msg)
	return msg
}

// CancelQueued removes and returns the queued message for a session.
func (s *QueuedMessageService) CancelQueued(sessionID domain.UUID) *QueuedMessage {
	v, ok := s.queue.LoadAndDelete(sessionID)
	if !ok {
		return nil
	}
	msg := v.(QueuedMessage)
	return &msg
}

// GetQueued returns (without removing) the queued message for a session.
func (s *QueuedMessageService) GetQueued(sessionID domain.UUID) *QueuedMessage {
	v, ok := s.queue.Load(sessionID)
	if !ok {
		return nil
	}
	msg := v.(QueuedMessage)
	return &msg
}

// TakeQueued removes and returns the queued message for a session.
func (s *QueuedMessageService) TakeQueued(sessionID domain.UUID) *QueuedMessage {
	v, ok := s.queue.LoadAndDelete(sessionID)
	if !ok {
		return nil
	}
	msg := v.(QueuedMessage)
	return &msg
}

// HasQueued checks if a session has a queued message.
func (s *QueuedMessageService) HasQueued(sessionID domain.UUID) bool {
	_, ok := s.queue.Load(sessionID)
	return ok
}

// GetStatus returns the queue status for a session.
func (s *QueuedMessageService) GetStatus(sessionID domain.UUID) QueueStatus {
	v, ok := s.queue.Load(sessionID)
	if !ok {
		return QueueStatus{IsEmpty: true}
	}
	msg := v.(QueuedMessage)
	return QueueStatus{IsEmpty: false, Msg: &msg}
}
