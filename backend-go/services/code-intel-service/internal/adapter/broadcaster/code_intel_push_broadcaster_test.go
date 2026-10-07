package broadcaster

import (
	"testing"
	"time"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

func TestCodeIntelPushBroadcaster_PublishAndOverflow(t *testing.T) {
	b := NewCodeIntelPushBroadcaster()

	ch, unsubscribe := b.Subscribe()
	defer unsubscribe()

	// 1. Normal publish
	b.Publish(&codeintelv1.CodeIntelPush{Kind: "changed", WorktreeId: "wt-1"})

	select {
	case p := <-ch:
		if p.WorktreeId != "wt-1" {
			t.Errorf("expected wt-1, got %s", p.WorktreeId)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for push")
	}

	// 2. Fill buffer up to 64
	for i := 0; i < 64; i++ {
		b.Publish(&codeintelv1.CodeIntelPush{Kind: "changed", EventId: "event-fill"})
	}

	// 65th publish triggers overflow message
	b.Publish(&codeintelv1.CodeIntelPush{Kind: "changed", EventId: "event-overflow-trigger"})

	// Drain messages
	var received []*codeintelv1.CodeIntelPush
	for len(ch) > 0 {
		received = append(received, <-ch)
	}

	if len(received) != 64 {
		t.Fatalf("expected 64 buffered messages, got %d", len(received))
	}
}

func TestCodeIntelPushBroadcaster_Unsubscribe(t *testing.T) {
	b := NewCodeIntelPushBroadcaster()

	ch, unsubscribe := b.Subscribe()
	unsubscribe()

	select {
	case _, ok := <-ch:
		if ok {
			t.Errorf("expected channel to be closed on unsubscribe")
		}
	default:
		t.Errorf("expected channel closed immediately")
	}

	b.mu.RLock()
	subsLen := len(b.subs)
	b.mu.RUnlock()
	if subsLen != 0 {
		t.Errorf("expected 0 subscribers left, got %d", subsLen)
	}
}
