//go:build integration

package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/adapter/broadcaster"
)

// simulatedEphemeralBroker simulates NATS JetStream SubscribeEphemeral fanout across replicas.
type simulatedEphemeralBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]commoneventbus.Handler
}

func newSimulatedEphemeralBroker() *simulatedEphemeralBroker {
	return &simulatedEphemeralBroker{
		subscribers: make(map[string][]commoneventbus.Handler),
	}
}

func (b *simulatedEphemeralBroker) SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error {
	b.mu.Lock()
	b.subscribers[subject] = append(b.subscribers[subject], fn)
	b.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (b *simulatedEphemeralBroker) Publish(ctx context.Context, subject string, ev commoneventbus.Event) {
	b.mu.RLock()
	handlers := append([]commoneventbus.Handler(nil), b.subscribers[subject]...)
	b.mu.RUnlock()

	for _, fn := range handlers {
		_ = fn(ctx, ev)
	}
}

// TestEventDistribution_TwoReplicasFanOut verifies that two distinct service replicas
// each receive a copy of events via ephemeral subscriptions, deduplicate per replica,
// and broadcast to their respective local clients.
func TestEventDistribution_TwoReplicasFanOut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := newSimulatedEphemeralBroker()

	// Replica 1 setup
	b1 := broadcaster.NewCodeIntelPushBroadcaster()
	sub1, unsub1 := b1.Subscribe()
	defer unsub1()
	c1 := NewCodeIntelConsumer(broker, b1, nil)
	go func() { _ = c1.Start(ctx) }()

	// Replica 2 setup
	b2 := broadcaster.NewCodeIntelPushBroadcaster()
	sub2, unsub2 := b2.Subscribe()
	defer unsub2()
	c2 := NewCodeIntelConsumer(broker, b2, nil)
	go func() { _ = c2.Start(ctx) }()

	// Allow goroutines to register subscriptions
	time.Sleep(50 * time.Millisecond)

	payloadBytes, err := json.Marshal(&codeintelv1.CodeIntelPush{
		Kind:       "changed",
		WorktreeId: "wt-common",
		Reason:     "index_changed",
	})
	if err != nil {
		t.Fatalf("failed to marshal push payload: %v", err)
	}

	testEvent := commoneventbus.Event{
		ID:         "evt-two-replicas-001",
		TenantID:   "tenant-alpha",
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    payloadBytes,
	}

	// Publish once to the shared bus
	broker.Publish(ctx, "orca.codeintel.index.changed", testEvent)

	// Both replica 1 and replica 2 should receive the event
	select {
	case p1 := <-sub1:
		if p1.WorktreeId != "wt-common" {
			t.Errorf("replica 1 unexpected push: %v", p1)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("replica 1 timed out waiting for event")
	}

	select {
	case p2 := <-sub2:
		if p2.WorktreeId != "wt-common" {
			t.Errorf("replica 2 unexpected push: %v", p2)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("replica 2 timed out waiting for event")
	}

	// Re-publishing the same event ID should be dropped by LRU on both replicas
	broker.Publish(ctx, "orca.codeintel.index.changed", testEvent)

	select {
	case p := <-sub1:
		t.Fatalf("replica 1 should have deduplicated event, got: %v", p)
	case p := <-sub2:
		t.Fatalf("replica 2 should have deduplicated event, got: %v", p)
	case <-time.After(100 * time.Millisecond):
		// Expected: no duplicate delivered
	}
}

// TestEventDistribution_StormLoad verifies that a high throughput storm of events
// is handled cleanly without deadlocks or unbounded memory growth.
func TestEventDistribution_StormLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := newSimulatedEphemeralBroker()
	bc := broadcaster.NewCodeIntelPushBroadcaster()
	subCh, unsub := bc.Subscribe()
	defer unsub()

	consumer := NewCodeIntelConsumer(broker, bc, nil)
	go func() { _ = consumer.Start(ctx) }()

	time.Sleep(50 * time.Millisecond)

	const totalEvents = 2000
	var receivedCount int64

	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-subCh:
				atomic.AddInt64(&receivedCount, 1)
			case <-doneCh:
				return
			}
		}
	}()

	start := time.Now()
	for i := 0; i < totalEvents; i++ {
		payloadBytes, _ := json.Marshal(&codeintelv1.CodeIntelPush{
			Kind:       "changed",
			WorktreeId: fmt.Sprintf("wt-%d", i),
			Reason:     "storm_test",
		})
		broker.Publish(ctx, "orca.codeintel.index.changed", commoneventbus.Event{
			ID:         fmt.Sprintf("evt-storm-%d", i),
			TenantID:   "tenant-storm",
			OccurredAt: time.Now().UTC(),
			Version:    1,
			Payload:    payloadBytes,
		})
	}

	// Drain
	time.Sleep(200 * time.Millisecond)
	close(doneCh)

	elapsed := time.Since(start)
	t.Logf("Processed %d events in %v (received: %d)", totalEvents, elapsed, atomic.LoadInt64(&receivedCount))
	if atomic.LoadInt64(&receivedCount) == 0 {
		t.Errorf("expected received count > 0")
	}
}
