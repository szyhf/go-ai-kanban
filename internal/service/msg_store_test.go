package service

import (
	"sync"
	"testing"
	"time"
)

func TestMsgStore_PushAndHistory(t *testing.T) {
	store := NewMsgStore()

	store.Push(NewStdoutLogMsg("hello"))
	store.Push(NewStderrLogMsg("error"))
	store.Push(NewReadyLogMsg())

	history := store.History()
	if len(history) != 3 {
		t.Fatalf("expected 3 history items, got %d", len(history))
	}
	if history[0].Kind != LogMsgStdout || history[0].Data != "hello" {
		t.Errorf("first msg: got %+v", history[0])
	}
	if history[1].Kind != LogMsgStderr || history[1].Data != "error" {
		t.Errorf("second msg: got %+v", history[1])
	}
	if history[2].Kind != LogMsgReady {
		t.Errorf("third msg: got %+v", history[2])
	}
}

func TestMsgStore_PushPatch(t *testing.T) {
	store := NewMsgStore()

	store.PushPatch(PatchOperation{Op: "add", Path: "/workspaces/123", Value: map[string]string{"name": "test"}})

	history := store.History()
	if len(history) != 1 {
		t.Fatal("expected 1 history item")
	}
	if history[0].Kind != LogMsgPatch {
		t.Error("expected patch kind")
	}
	if history[0].Patch == nil {
		t.Fatal("expected non-nil patch")
	}
	if history[0].Patch.Op != "add" {
		t.Errorf("expected add op, got %s", history[0].Patch.Op)
	}
}

func TestMsgStore_Subscribe(t *testing.T) {
	store := NewMsgStore()

	ch, unsub := store.Subscribe()
	defer unsub()

	store.Push(NewStdoutLogMsg("msg1"))
	store.Push(NewStdoutLogMsg("msg2"))

	// Read from channel with timeout.
	msgs := drainChannel(ch, 2, time.Second)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Data != "msg1" {
		t.Errorf("first msg: got %q", msgs[0].Data)
	}
	if msgs[1].Data != "msg2" {
		t.Errorf("second msg: got %q", msgs[1].Data)
	}
}

func TestMsgStore_MultipleSubscribers(t *testing.T) {
	store := NewMsgStore()

	ch1, unsub1 := store.Subscribe()
	ch2, unsub2 := store.Subscribe()
	defer unsub1()
	defer unsub2()

	store.Push(NewStdoutLogMsg("broadcast"))

	msgs1 := drainChannel(ch1, 1, time.Second)
	msgs2 := drainChannel(ch2, 1, time.Second)

	if len(msgs1) != 1 || msgs1[0].Data != "broadcast" {
		t.Error("subscriber 1 did not receive message")
	}
	if len(msgs2) != 1 || msgs2[0].Data != "broadcast" {
		t.Error("subscriber 2 did not receive message")
	}
}

func TestMsgStore_Unsubscribe(t *testing.T) {
	store := NewMsgStore()

	ch, unsub := store.Subscribe()

	store.Push(NewStdoutLogMsg("before"))
	unsub()
	store.Push(NewStdoutLogMsg("after"))

	msgs := drainChannel(ch, 2, 100*time.Millisecond)
	// Should only get "before" since we unsubscribed before "after".
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Data != "before" {
		t.Errorf("got %q", msgs[0].Data)
	}
}

func TestMsgStore_SubscriberCount(t *testing.T) {
	store := NewMsgStore()

	if store.SubscriberCount() != 0 {
		t.Error("expected 0 subscribers")
	}

	_, unsub1 := store.Subscribe()
	if store.SubscriberCount() != 1 {
		t.Error("expected 1 subscriber")
	}

	_, unsub2 := store.Subscribe()
	if store.SubscriberCount() != 2 {
		t.Error("expected 2 subscribers")
	}

	unsub1()
	if store.SubscriberCount() != 1 {
		t.Error("expected 1 subscriber after unsub")
	}

	unsub2()
	if store.SubscriberCount() != 0 {
		t.Error("expected 0 subscribers after all unsub")
	}
}

func TestMsgStore_Concurrent(t *testing.T) {
	store := NewMsgStore()
	var wg sync.WaitGroup

	// Start 3 subscribers.
	for i := 0; i < 3; i++ {
		ch, unsub := store.Subscribe()
		defer unsub()
		wg.Add(1)
		go func() {
			defer wg.Done()
			drainChannel(ch, 100, 5*time.Second)
		}()
	}

	// Push 100 messages from multiple goroutines.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				store.Push(NewStdoutLogMsg("msg"))
			}
		}(i)
	}

	wg.Wait()
}

func TestLogMsg_ToSSEEvent(t *testing.T) {
	tests := []struct {
		name      string
		msg       LogMsg
		wantEvent string
	}{
		{"stdout", NewStdoutLogMsg("hello"), "stdout"},
		{"stderr", NewStderrLogMsg("err"), "stderr"},
		{"ready", NewReadyLogMsg(), "ready"},
		{"finished", NewFinishedLogMsg(), "finished"},
		{"patch", NewPatchLogMsg(PatchOperation{Op: "add", Path: "/x"}), "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _ := tt.msg.ToSSEEvent()
			if event != tt.wantEvent {
				t.Errorf("got event %q, want %q", event, tt.wantEvent)
			}
		})
	}
}

// drainChannel reads up to n messages from a channel with a timeout.
func drainChannel(ch <-chan LogMsg, n int, timeout time.Duration) []LogMsg {
	var msgs []LogMsg
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for len(msgs) < n {
		select {
		case msg, ok := <-ch:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		case <-timer.C:
			return msgs
		}
	}
	return msgs
}
