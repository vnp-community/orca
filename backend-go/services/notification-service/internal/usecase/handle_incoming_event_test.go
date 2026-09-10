package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// noopDeliverPush builds a *DeliverPush that's safe to pass to every test
// in this file that isn't specifically about push delivery: its
// VapidKeyRepository always errors, so DeliverPush.Execute logs and
// returns nil immediately (see deliver_push.go's GetPublicKey error
// branch) — never touching the nil signer/sender fields, regardless of
// whether a given test's translated event happens to have
// domain.ChannelDeliveryPush in its Channels.
func noopDeliverPush() *DeliverPush {
	return NewDeliverPush(&fakeSubscriptionRepository{}, &fakeVapidKeyRepository{err: errors.New("no vapid key configured (test)")}, nil, nil, "", nil)
}

// noopDeliverMobilePush is DeliverMobilePush's counterpart to
// noopDeliverPush above — an empty senders map means Execute's per-channel
// lookup always misses (`ok == false`), so it's a safe no-op regardless of
// whether a test's translated event has domain.ChannelDeliveryPush.
func noopDeliverMobilePush() *DeliverMobilePush {
	return NewDeliverMobilePush(&fakeSubscriptionRepository{}, map[domain.Channel]PushSender{}, nil)
}

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

// fakeNotificationRepository is an in-memory NotificationRepository shared
// across this package's usecase tests (HandleIncomingEvent's
// save-before-broadcast ordering here, and TASK-BE-NOTIF-005's CRUD
// usecases in list_notifications_test.go/mark_as_read_test.go/
// mark_all_as_read_test.go/get_unread_count_test.go) — one fake, one
// definition, per the "test against fakes" convention already used by
// fakeSubscriptionRepository. Each method records the args it was called
// with (last* fields) so a test can assert the usecase passed through the
// right tenantID/userID/etc without reaching for a mock framework.
type fakeNotificationRepository struct {
	saveErr error
	saved   []domain.NotificationEvent
	order   *[]string // shared with a broadcaster fake to assert call order

	listEvents                                       []domain.NotificationEvent
	listCursor                                       string
	listErr                                          error
	lastListTenantID, lastListUserID, lastListCursor string
	lastListLimit                                    int32
	lastListUnreadOnly                               bool

	markAsReadErr                                                  error
	lastMarkAsReadTenantID, lastMarkAsReadUserID, lastMarkAsReadID string

	markAllAsReadCount                     int64
	markAllAsReadErr                       error
	lastMarkAllTenantID, lastMarkAllUserID string

	countUnreadResult                  int64
	countUnreadErr                     error
	lastCountTenantID, lastCountUserID string
}

func (f *fakeNotificationRepository) SaveNotificationEvent(ctx context.Context, event domain.NotificationEvent) error {
	if f.order != nil {
		*f.order = append(*f.order, "save")
	}
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, event)
	return nil
}

func (f *fakeNotificationRepository) ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error) {
	f.lastListTenantID, f.lastListUserID, f.lastListCursor = tenantID, userID, cursor
	f.lastListLimit, f.lastListUnreadOnly = limit, unreadOnly
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listEvents, f.listCursor, nil
}

func (f *fakeNotificationRepository) MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error {
	f.lastMarkAsReadTenantID, f.lastMarkAsReadUserID, f.lastMarkAsReadID = tenantID, userID, notificationID
	return f.markAsReadErr
}

func (f *fakeNotificationRepository) MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error) {
	f.lastMarkAllTenantID, f.lastMarkAllUserID = tenantID, userID
	if f.markAllAsReadErr != nil {
		return 0, f.markAllAsReadErr
	}
	return f.markAllAsReadCount, nil
}

func (f *fakeNotificationRepository) CountUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	f.lastCountTenantID, f.lastCountUserID = tenantID, userID
	if f.countUnreadErr != nil {
		return 0, f.countUnreadErr
	}
	return f.countUnreadResult, nil
}

// orderedFakeBroadcaster wraps fakeBroadcaster to also append to a shared
// order slice, so TestHandleIncomingEvent_SavesBeforeBroadcast can assert
// call order across both fakes.
type orderedFakeBroadcaster struct {
	fakeBroadcaster
	order *[]string
}

func (f *orderedFakeBroadcaster) Broadcast(ctx context.Context, event domain.NotificationEvent) {
	if f.order != nil {
		*f.order = append(*f.order, "broadcast")
	}
	f.fakeBroadcaster.Broadcast(ctx, event)
}

func TestHandleIncomingEvent_TranslatesAndBroadcasts(t *testing.T) {
	b := &fakeBroadcaster{}
<<<<<<< HEAD
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, nil, nil)
=======
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, noopDeliverPush(), noopDeliverMobilePush(), nil)
>>>>>>> feat/team-rbac-implementation

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
<<<<<<< HEAD
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, nil, nil)
=======
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, noopDeliverPush(), noopDeliverMobilePush(), nil)
>>>>>>> feat/team-rbac-implementation

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
<<<<<<< HEAD
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, nil, nil)
=======
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, noopDeliverPush(), noopDeliverMobilePush(), nil)
>>>>>>> feat/team-rbac-implementation

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
<<<<<<< HEAD
	uc := NewHandleIncomingEvent(b, dedup, nil, nil)
=======
	uc := NewHandleIncomingEvent(b, dedup, &fakeNotificationRepository{}, noopDeliverPush(), noopDeliverMobilePush(), nil)
>>>>>>> feat/team-rbac-implementation

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
<<<<<<< HEAD
	uc := NewHandleIncomingEvent(b, dedup, nil, nil)
=======
	uc := NewHandleIncomingEvent(b, dedup, &fakeNotificationRepository{}, noopDeliverPush(), noopDeliverMobilePush(), nil)
>>>>>>> feat/team-rbac-implementation

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

// TestHandleIncomingEvent_SavesBeforeBroadcast verifies the CR-NOTIF-001
// ordering rule: SaveNotificationEvent must complete before Broadcast is
// invoked, so a crash between the two leaves a persisted record rather
// than a lost one.
func TestHandleIncomingEvent_SavesBeforeBroadcast(t *testing.T) {
	var order []string
	b := &orderedFakeBroadcaster{order: &order}
	notifications := &fakeNotificationRepository{order: &order}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, notifications, noopDeliverPush(), noopDeliverMobilePush(), nil)

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

	want := []string{"save", "broadcast"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("expected call order %v, got %v", want, order)
	}
}

// TestHandleIncomingEvent_SaveFails_ReturnsErrorNoBroadcast verifies the
// other half of the ordering rule: a persist failure must abort Execute
// (so the eventbus adapter NAKs for JetStream redelivery) and must NOT
// still broadcast — persist success is the only path that's allowed to
// reach subscribers.
func TestHandleIncomingEvent_SaveFails_ReturnsErrorNoBroadcast(t *testing.T) {
	b := &fakeBroadcaster{}
	notifications := &fakeNotificationRepository{saveErr: errors.New("persist boom")}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, notifications, noopDeliverPush(), noopDeliverMobilePush(), nil)

	err := uc.Execute(context.Background(), HandleIncomingEventInput{
		EventID:    "evt-1",
		TenantID:   "tenant-1",
		Subject:    "orca.task.task.completed",
		OccurredAt: time.Now(),
		Payload:    []byte(`{"user_id":"user-1"}`),
	})
	if err == nil {
		t.Fatal("expected an error when SaveNotificationEvent fails")
	}
	if len(b.broadcast) != 0 {
		t.Errorf("expected Broadcast NOT called when persist fails, got %d calls", len(b.broadcast))
	}
}

// TestHandleIncomingEvent_NoRecipients_SkipsSaveAndBroadcast is a
// regression guard for adding the notifications field: ErrNoRecipients
// must still short-circuit before both Save and Broadcast, not just
// Broadcast.
func TestHandleIncomingEvent_NoRecipients_SkipsSaveAndBroadcast(t *testing.T) {
	b := &fakeBroadcaster{}
	notifications := &fakeNotificationRepository{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, notifications, noopDeliverPush(), noopDeliverMobilePush(), nil)

	err := uc.Execute(context.Background(), HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: time.Now(), Payload: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("expected no error for a no-recipient event, got %v", err)
	}
	if len(notifications.saved) != 0 {
		t.Errorf("expected SaveNotificationEvent NOT called for a no-recipient event, got %d calls", len(notifications.saved))
	}
	if len(b.broadcast) != 0 {
		t.Errorf("expected Broadcast NOT called for a no-recipient event, got %d calls", len(b.broadcast))
	}
}
