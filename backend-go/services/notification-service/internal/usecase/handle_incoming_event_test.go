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

func TestHandleIncomingEvent_TranslatesAndBroadcasts(t *testing.T) {
	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, nil)

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
	uc := NewHandleIncomingEvent(b, nil)

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
	uc := NewHandleIncomingEvent(b, nil)

	err := uc.Execute(context.Background(), HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: time.Now(), Payload: []byte(`not-json`),
	})
	if err == nil {
		t.Fatal("expected an error for a malformed payload")
	}
}
