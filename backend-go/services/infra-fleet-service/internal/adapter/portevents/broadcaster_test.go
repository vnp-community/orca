package portevents

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TestBroadcaster_PublishDeliversOnlyToMatchingConnectionSubscribers is this
// task's core assertion: a Publish("dev_server.port_opened", pf) call
// delivers to every Subscribe(pf.ConnectionID) listener and none of any
// other connectionId's.
func TestBroadcaster_PublishDeliversOnlyToMatchingConnectionSubscribers(t *testing.T) {
	b := NewBroadcaster()

	matching, unsubMatching := b.Subscribe("conn-1")
	defer unsubMatching()
	other, unsubOther := b.Subscribe("conn-2")
	defer unsubOther()

	pf := domain.PortForward{ID: "pf-1", ConnectionID: "conn-1", LocalPort: 3001, RemotePort: 3000, ProcessName: "node"}
	b.Publish(context.Background(), "dev_server.port_opened", pf)

	select {
	case ev := <-matching:
		if ev.Kind != "opened" {
			t.Fatalf("event.Kind = %q, want %q", ev.Kind, "opened")
		}
		if ev.Forward.ID != "pf-1" {
			t.Fatalf("event.Forward.ID = %q, want %q", ev.Forward.ID, "pf-1")
		}
	case <-time.After(time.Second):
		t.Fatal("matching subscriber did not receive the published event")
	}

	select {
	case ev := <-other:
		t.Fatalf("subscriber for a different connectionId received an event: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected — no delivery to conn-2's subscriber
	}
}

// TestBroadcaster_PublishMapsClosedEventKind verifies the
// "dev_server.port_closed" event string is a translated to Kind="closed"
// (every other event value, including "dev_server.port_opened", maps to
// "opened").
func TestBroadcaster_PublishMapsClosedEventKind(t *testing.T) {
	b := NewBroadcaster()
	ch, unsubscribe := b.Subscribe("conn-1")
	defer unsubscribe()

	pf := domain.PortForward{ID: "pf-1", ConnectionID: "conn-1"}
	b.Publish(context.Background(), "dev_server.port_closed", pf)

	select {
	case ev := <-ch:
		if ev.Kind != "closed" {
			t.Fatalf("event.Kind = %q, want %q", ev.Kind, "closed")
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive the published event")
	}
}

// TestBroadcaster_PublishDropsOnFullSubscriberBuffer verifies the "drop
// rather than block PollWorkspacePorts" discipline the package doc comment
// promises: once a subscriber's buffered channel (capacity 16) is full,
// Publish must not block.
func TestBroadcaster_PublishDropsOnFullSubscriberBuffer(t *testing.T) {
	b := NewBroadcaster()
	ch, unsubscribe := b.Subscribe("conn-1")
	defer unsubscribe()

	pf := domain.PortForward{ID: "pf-1", ConnectionID: "conn-1"}
	for i := 0; i < 16; i++ {
		b.Publish(context.Background(), "dev_server.port_opened", pf)
	}
	if n := len(ch); n != 16 {
		t.Fatalf("buffered subscriber channel has %d items, want 16 (full)", n)
	}

	done := make(chan struct{})
	go func() {
		b.Publish(context.Background(), "dev_server.port_opened", pf) // 17th — buffer full
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber instead of dropping (select default: branch)")
	}
}

// TestBroadcaster_UnsubscribeStopsDeliveryAndClosesChannel verifies the
// returned unsubscribe func removes the listener from subs and closes its
// channel exactly once.
func TestBroadcaster_UnsubscribeStopsDeliveryAndClosesChannel(t *testing.T) {
	b := NewBroadcaster()
	ch, unsubscribe := b.Subscribe("conn-1")
	unsubscribe()

	pf := domain.PortForward{ID: "pf-1", ConnectionID: "conn-1"}
	b.Publish(context.Background(), "dev_server.port_opened", pf)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected the channel to be closed after unsubscribe, got a delivered event")
		}
	case <-time.After(time.Second):
		t.Fatal("expected the unsubscribed channel to be closed, but reading blocked")
	}

	b.mu.Lock()
	n := len(b.subs["conn-1"])
	b.mu.Unlock()
	if n != 0 {
		t.Fatalf("subs[\"conn-1\"] has %d entries after unsubscribe, want 0", n)
	}
}
