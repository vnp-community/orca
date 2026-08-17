package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// fakeBroadcaster records every Broadcast call — used to verify
// HandleIncomingEvent's translation-then-broadcast wiring without a real
// channel-based fan-out (that fan-out itself is exercised directly against
// internal/adapter/broadcaster.Broadcaster).
type fakeBroadcaster struct {
	broadcast []domain.NotificationEvent
}

func (f *fakeBroadcaster) Subscribe(ctx context.Context, tenantID, userID string) (<-chan domain.NotificationEvent, func()) {
	ch := make(chan domain.NotificationEvent)
	return ch, func() { close(ch) }
}

func (f *fakeBroadcaster) Broadcast(ctx context.Context, event domain.NotificationEvent) {
	f.broadcast = append(f.broadcast, event)
}

// fakeProcessedEventRepository mirrors ProcessedEventRepository's atomic
// reserve-on-first-call semantics in memory — good enough to exercise
// HandleIncomingEvent's dedup wiring without a real Postgres connection.
type fakeProcessedEventRepository struct {
	seen  map[string]bool
	calls int
}

func (f *fakeProcessedEventRepository) MarkProcessed(ctx context.Context, eventID, subject string) (bool, error) {
	f.calls++
	if f.seen == nil {
		f.seen = make(map[string]bool)
	}
	if f.seen[eventID] {
		return true, nil
	}
	f.seen[eventID] = true
	return false, nil
}

func TestHandleIncomingEvent_TranslatesAndBroadcasts(t *testing.T) {
	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, nil)

	err := uc.Execute(context.Background(), HandleIncomingEventInput{
		EventID:    "evt-1",
		TenantID:   "tenant-1",
		Subject:    "orca.task.task.completed",
		OccurredAt: time.Now(),
		Payload:    []byte(`{"user_id":"user-1"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b.broadcast) != 1 {
		t.Fatalf("expected 1 broadcast event, got %d", len(b.broadcast))
	}
	got := b.broadcast[0]
	if got.TenantID != "tenant-1" || got.SourceEventID != "evt-1" || got.Type != "task_completed" {
		t.Errorf("unexpected translated event: %+v", got)
	}
}

func TestHandleIncomingEvent_NoRecipientsIsANoOpNotAnError(t *testing.T) {
	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, nil)

	err := uc.Execute(context.Background(), HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: time.Now(), Payload: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("expected no error for a no-recipient event, got %v", err)
	}
	if len(b.broadcast) != 0 {
		t.Errorf("expected no broadcast for a no-recipient event, got %d", len(b.broadcast))
	}
}

func TestHandleIncomingEvent_MalformedPayloadReturnsError(t *testing.T) {
	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, nil)

	err := uc.Execute(context.Background(), HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: time.Now(), Payload: []byte(`not-json`),
	})
	if err == nil {
		t.Fatal("expected an error for a malformed payload")
	}
}

// TestHandleIncomingEvent_RedeliveryOfSameEventIDIsANoOp verifies
// JetStream's at-least-once redelivery of the same event ID doesn't
// double-broadcast — the dedup check in Execute must short-circuit before
// translation/broadcast on the second delivery.
func TestHandleIncomingEvent_RedeliveryOfSameEventIDIsANoOp(t *testing.T) {
	b := &fakeBroadcaster{}
	dedup := &fakeProcessedEventRepository{}
	uc := NewHandleIncomingEvent(b, dedup, nil)

	input := HandleIncomingEventInput{
		EventID:    "evt-redelivered",
		TenantID:   "tenant-1",
		Subject:    "orca.task.task.completed",
		OccurredAt: time.Now(),
		Payload:    []byte(`{"user_id":"user-1"}`),
	}

	if err := uc.Execute(context.Background(), input); err != nil {
		t.Fatalf("first delivery: unexpected error: %v", err)
	}
	if len(b.broadcast) != 1 {
		t.Fatalf("first delivery: expected 1 broadcast, got %d", len(b.broadcast))
	}

	// Redelivery of the exact same event ID (e.g. JetStream redelivering
	// after a slow ack, or another replica's independent SubscribeEphemeral
	// consumer racing the same message).
	if err := uc.Execute(context.Background(), input); err != nil {
		t.Fatalf("redelivery: expected a no-op success, got error: %v", err)
	}
	if len(b.broadcast) != 1 {
		t.Errorf("redelivery: expected broadcaster NOT called again, still got %d broadcasts", len(b.broadcast))
	}
	if dedup.calls != 2 {
		t.Errorf("expected MarkProcessed called once per delivery attempt, got %d calls", dedup.calls)
	}
}

// TestHandleIncomingEvent_DifferentEventIDsBothProcess verifies dedup is
// keyed per event ID, not a global gate — two distinct events must both
// broadcast normally.
func TestHandleIncomingEvent_DifferentEventIDsBothProcess(t *testing.T) {
	b := &fakeBroadcaster{}
	dedup := &fakeProcessedEventRepository{}
	uc := NewHandleIncomingEvent(b, dedup, nil)

	for _, eventID := range []string{"evt-a", "evt-b"} {
		err := uc.Execute(context.Background(), HandleIncomingEventInput{
			EventID:    eventID,
			TenantID:   "tenant-1",
			Subject:    "orca.task.task.completed",
			OccurredAt: time.Now(),
			Payload:    []byte(`{"user_id":"user-1"}`),
		})
		if err != nil {
			t.Fatalf("event %s: unexpected error: %v", eventID, err)
		}
	}

	if len(b.broadcast) != 2 {
		t.Fatalf("expected 2 broadcasts (one per distinct event ID), got %d", len(b.broadcast))
	}
}
