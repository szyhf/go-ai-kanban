package service

import (
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
)

func TestQueuedMessageService_QueueAndTake(t *testing.T) {
	svc := NewQueuedMessageService()
	sid := domain.NewUUID()

	data := domain.DraftFollowUpData{Message: "hello"}
	msg := svc.QueueMessage(sid, data)

	if msg.SessionID != sid {
		t.Error("SessionID mismatch")
	}
	if msg.Data.Message != "hello" {
		t.Error("Data.Message mismatch")
	}
	if msg.QueuedAt.IsZero() {
		t.Error("QueuedAt should not be zero")
	}

	if !svc.HasQueued(sid) {
		t.Error("should have queued message")
	}

	// Take the message.
	got := svc.TakeQueued(sid)
	if got == nil {
		t.Fatal("expected queued message")
	}
	if got.Data.Message != "hello" {
		t.Error("taken message data mismatch")
	}

	// Should be gone now.
	if svc.HasQueued(sid) {
		t.Error("should not have queued message after take")
	}
	if svc.TakeQueued(sid) != nil {
		t.Error("should return nil after take")
	}
}

func TestQueuedMessageService_Cancel(t *testing.T) {
	svc := NewQueuedMessageService()
	sid := domain.NewUUID()

	svc.QueueMessage(sid, domain.DraftFollowUpData{Message: "test"})
	got := svc.CancelQueued(sid)
	if got == nil {
		t.Fatal("expected cancelled message")
	}
	if got.Data.Message != "test" {
		t.Error("cancelled message data mismatch")
	}

	if svc.HasQueued(sid) {
		t.Error("should not have queued message after cancel")
	}
}

func TestQueuedMessageService_GetStatus(t *testing.T) {
	svc := NewQueuedMessageService()
	sid := domain.NewUUID()

	status := svc.GetStatus(sid)
	if !status.IsEmpty {
		t.Error("expected empty status")
	}

	svc.QueueMessage(sid, domain.DraftFollowUpData{Message: "test"})
	status = svc.GetStatus(sid)
	if status.IsEmpty {
		t.Error("expected non-empty status")
	}
	if status.Msg == nil || status.Msg.Data.Message != "test" {
		t.Error("status message mismatch")
	}
}

func TestQueuedMessageService_ReplaceExisting(t *testing.T) {
	svc := NewQueuedMessageService()
	sid := domain.NewUUID()

	svc.QueueMessage(sid, domain.DraftFollowUpData{Message: "first"})
	svc.QueueMessage(sid, domain.DraftFollowUpData{Message: "second"})

	got := svc.TakeQueued(sid)
	if got == nil {
		t.Fatal("expected message")
	}
	if got.Data.Message != "second" {
		t.Errorf("expected second message, got %q", got.Data.Message)
	}
}

func TestQueuedMessageService_GetWithoutRemove(t *testing.T) {
	svc := NewQueuedMessageService()
	sid := domain.NewUUID()

	svc.QueueMessage(sid, domain.DraftFollowUpData{Message: "peek"})

	got := svc.GetQueued(sid)
	if got == nil || got.Data.Message != "peek" {
		t.Error("GetQueued should return message without removing")
	}

	if !svc.HasQueued(sid) {
		t.Error("message should still be queued after GetQueued")
	}
}
