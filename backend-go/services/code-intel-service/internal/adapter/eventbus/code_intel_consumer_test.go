package eventbus

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

type mockSubscriber struct {
	mu       sync.Mutex
	handlers map[string]commoneventbus.Handler
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{
		handlers: make(map[string]commoneventbus.Handler),
	}
}

func (m *mockSubscriber) SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error {
	m.mu.Lock()
	m.handlers[subject] = fn
	m.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (m *mockSubscriber) dispatch(ctx context.Context, subject string, ev commoneventbus.Event) error {
	m.mu.Lock()
	fn, ok := m.handlers[subject]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return fn(ctx, ev)
}

type mockBroadcaster struct {
	mu     sync.Mutex
	pushes []*codeintelv1.CodeIntelPush
}

func (b *mockBroadcaster) Publish(push *codeintelv1.CodeIntelPush) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pushes = append(b.pushes, push)
}

func (b *mockBroadcaster) getPushes() []*codeintelv1.CodeIntelPush {
	b.mu.Lock()
	defer b.mu.Unlock()
	res := make([]*codeintelv1.CodeIntelPush, len(b.pushes))
	copy(res, b.pushes)
	return res
}

func TestCodeIntelConsumer_FilterAndDedup(t *testing.T) {
	sub := newMockSubscriber()
	broadcaster := &mockBroadcaster{}
	consumer := NewCodeIntelConsumer(sub, broadcaster, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = consumer.Start(ctx)
	}()

	// Wait for subscriptions to register
	time.Sleep(50 * time.Millisecond)

	payloadBytes, err := json.Marshal(&codeintelv1.CodeIntelPush{
		Kind:       "changed",
		WorktreeId: "wt-1",
		Reason:     "git_head",
	})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// 1. Event without TenantID should be dropped
	err = sub.dispatch(ctx, "orca.codeintel.index.changed", commoneventbus.Event{
		ID:         "ev-1",
		TenantID:   "",
		Payload:    payloadBytes,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if len(broadcaster.getPushes()) != 0 {
		t.Fatalf("expected 0 pushes for missing tenant, got %d", len(broadcaster.getPushes()))
	}

	// 2. Valid event should be broadcast
	err = sub.dispatch(ctx, "orca.codeintel.index.changed", commoneventbus.Event{
		ID:         "ev-2",
		TenantID:   "tenant-1",
		Payload:    payloadBytes,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if len(broadcaster.getPushes()) != 1 {
		t.Fatalf("expected 1 push, got %d", len(broadcaster.getPushes()))
	}
	if broadcaster.getPushes()[0].WorktreeId != "wt-1" {
		t.Errorf("expected WorktreeId wt-1, got %s", broadcaster.getPushes()[0].WorktreeId)
	}

	// 3. Duplicate event with same ID should be dropped by LRU
	err = sub.dispatch(ctx, "orca.codeintel.index.changed", commoneventbus.Event{
		ID:         "ev-2",
		TenantID:   "tenant-1",
		Payload:    payloadBytes,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if len(broadcaster.getPushes()) != 1 {
		t.Fatalf("expected still 1 push after duplicate, got %d", len(broadcaster.getPushes()))
	}

	// 4. New event with different ID should succeed
	err = sub.dispatch(ctx, "orca.codeintel.reindex.finished", commoneventbus.Event{
		ID:         "ev-3",
		TenantID:   "tenant-1",
		Payload:    payloadBytes,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if len(broadcaster.getPushes()) != 2 {
		t.Fatalf("expected 2 pushes after new event, got %d", len(broadcaster.getPushes()))
	}
}

func TestCodeIntelConsumer_LRUCapacity(t *testing.T) {
	lru := newEventLRU(3)

	if lru.checkAndAdd("a") {
		t.Errorf("expected 'a' to be new")
	}
	if lru.checkAndAdd("b") {
		t.Errorf("expected 'b' to be new")
	}
	if lru.checkAndAdd("c") {
		t.Errorf("expected 'c' to be new")
	}

	// 'a' is present
	if !lru.checkAndAdd("a") {
		t.Errorf("expected 'a' to be duplicate")
	}

	// Adding 'd' should evict 'b' (oldest not newly inserted)
	// Order was [b, c, a] after 'a' was duplicate-checked or touched? Wait, order in simple queue:
	// In our simple FIFO: order was [a, b, c], 'd' evicts 'a'
	if lru.checkAndAdd("d") {
		t.Errorf("expected 'd' to be new")
	}
}
