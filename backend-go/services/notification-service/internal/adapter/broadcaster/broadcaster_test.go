package broadcaster

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// Compile-time assertion that Broadcaster satisfies the usecase port.
var _ usecase.NotificationBroadcaster = (*Broadcaster)(nil)

// TestBroadcaster_FanOutIsPerUserIsolated is the fan-out test the design
// asks for: subscribe two "connections" for different users, publish an
// event for one user, assert only that user's channel receives it.
func TestBroadcaster_FanOutIsPerUserIsolated(t *testing.T) {
	b := New()
	ctx := context.Background()

	chUser1, unsubUser1 := b.Subscribe(ctx, "tenant-1", "user-1")
	defer unsubUser1()
	chUser2, unsubUser2 := b.Subscribe(ctx, "tenant-1", "user-2")
	defer unsubUser2()

	event := domain.NotificationEvent{
		ID:               "ne-1",
		TenantID:         "tenant-1",
		RecipientUserIDs: []string{"user-1"},
		Type:             "task_completed",
		Title:            "Task completed",
	}
	b.Broadcast(ctx, event)

	select {
	case got := <-chUser1:
		if got.ID != event.ID {
			t.Errorf("expected event %s, got %s", event.ID, got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for user-1's channel to receive the broadcast event")
	}

	select {
	case got := <-chUser2:
		t.Fatalf("expected user-2's channel to receive nothing, got %+v", got)
	case <-time.After(50 * time.Millisecond):
		// expected: user-2 was never a recipient
	}
}

func TestBroadcaster_TenantIsolation(t *testing.T) {
	b := New()
	ctx := context.Background()

	// Same user_id, different tenant — must not cross-deliver.
	chTenant1, unsub1 := b.Subscribe(ctx, "tenant-1", "user-1")
	defer unsub1()
	chTenant2, unsub2 := b.Subscribe(ctx, "tenant-2", "user-1")
	defer unsub2()

	b.Broadcast(ctx, domain.NotificationEvent{ID: "ne-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1"}})

	select {
	case <-chTenant1:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for tenant-1's subscriber to receive the event")
	}

	select {
	case got := <-chTenant2:
		t.Fatalf("expected tenant-2's subscriber to receive nothing, got %+v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcaster_MultipleSubscribersForSameUserBothReceive(t *testing.T) {
	b := New()
	ctx := context.Background()

	ch1, unsub1 := b.Subscribe(ctx, "tenant-1", "user-1")
	defer unsub1()
	ch2, unsub2 := b.Subscribe(ctx, "tenant-1", "user-1")
	defer unsub2()

	b.Broadcast(ctx, domain.NotificationEvent{ID: "ne-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1"}})

	for i, ch := range []<-chan domain.NotificationEvent{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for subscriber %d to receive the event", i)
		}
	}
}

func TestBroadcaster_UnsubscribeRemovesFromRegistry(t *testing.T) {
	b := New()
	ctx := context.Background()

	ch, unsubscribe := b.Subscribe(ctx, "tenant-1", "user-1")
	unsubscribe()

	if _, ok := <-ch; ok {
		t.Fatal("expected channel to be closed after unsubscribe")
	}

	// Broadcasting after unsubscribe must not panic (send-on-closed-channel
	// would panic if the registry still held the closed channel).
	b.Broadcast(ctx, domain.NotificationEvent{ID: "ne-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1"}})
}
