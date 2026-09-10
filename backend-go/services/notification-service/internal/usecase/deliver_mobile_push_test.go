package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// fakePushSender is an in-memory usecase.PushSender double, mirroring this
// package's other fakes — records every Send call and lets a test script
// per-endpoint outcomes.
type fakePushSender struct {
	sent   []string // endpoints Send was called with, in call order
	errFor map[string]error
}

func (f *fakePushSender) Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error {
	f.sent = append(f.sent, sub.Endpoint)
	if err, ok := f.errFor[sub.Endpoint]; ok {
		return err
	}
	return nil
}

func mobilePushEvent() domain.NotificationEvent {
	return domain.NotificationEvent{
		ID: "evt-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1"}, Title: "t", Body: "b",
		Channels: []domain.DeliveryChannel{domain.ChannelDeliveryWS, domain.ChannelDeliveryPush},
	}
}

// TestDeliverMobilePush_ChannelsWithoutPush_NoOp mirrors DeliverPush's own
// test of the same name (deliver_push_test.go, CR-NOTIF-001) — both push
// usecases share the same "check event.Channels" gate.
func TestDeliverMobilePush_ChannelsWithoutPush_NoOp(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{iosTestSubscription(t, "s1", "ios-token-1")}
	iosSender := &fakePushSender{}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	event := domain.NotificationEvent{ID: "evt-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1"}, Channels: []domain.DeliveryChannel{domain.ChannelDeliveryWS}}
	if err := uc.Execute(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 0 {
		t.Fatalf("expected 0 sends when Channels has no push, got %d", len(iosSender.sent))
	}
}

func TestDeliverMobilePush_SendsToEveryActiveIOSAndAndroidSubscription(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		webPushTestSubscription(t, "s1", "https://push.example/ep-1"), // web — no sender registered
		iosTestSubscription(t, "s2", "ios-token-1"),
		{ID: "s3", TenantID: "tenant-1", UserID: "user-1", Channel: domain.ChannelAndroid, Endpoint: "fcm-token-1", Status: domain.SubscriptionActive},
	}
	iosSender := &fakePushSender{}
	androidSender := &fakePushSender{}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{
		domain.ChannelIOS:     iosSender,
		domain.ChannelAndroid: androidSender,
	}, nil)

	if err := uc.Execute(context.Background(), mobilePushEvent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 1 || iosSender.sent[0] != "ios-token-1" {
		t.Errorf("expected ios sender called once with ios-token-1, got %v", iosSender.sent)
	}
	if len(androidSender.sent) != 1 || androidSender.sent[0] != "fcm-token-1" {
		t.Errorf("expected android sender called once with fcm-token-1, got %v", androidSender.sent)
	}
}

func TestDeliverMobilePush_SkipsWebChannelSubscriptions(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{webPushTestSubscription(t, "s1", "https://push.example/ep-1")}
	iosSender := &fakePushSender{}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	if err := uc.Execute(context.Background(), mobilePushEvent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 0 {
		t.Fatalf("expected 0 sends — web channel has no registered sender in this map, got %v", iosSender.sent)
	}
}

func TestDeliverMobilePush_DeviceTokenInvalidMarksExpiredAndContinues(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		iosTestSubscription(t, "s1", "dead-token"),
		iosTestSubscription(t, "s2", "live-token"),
	}
	iosSender := &fakePushSender{errFor: map[string]error{"dead-token": ErrDeviceTokenInvalid}}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	if err := uc.Execute(context.Background(), mobilePushEvent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 2 {
		t.Fatalf("expected both subscriptions attempted, got %d sends: %v", len(iosSender.sent), iosSender.sent)
	}
	if len(subs.markExpired) != 1 || subs.markExpired[0] != "dead-token" {
		t.Fatalf("expected MarkExpired called once with dead-token, got %v", subs.markExpired)
	}
}

func TestDeliverMobilePush_GenericSendErrorDoesNotMarkExpired(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		iosTestSubscription(t, "s1", "token-1"),
		iosTestSubscription(t, "s2", "token-2"),
	}
	iosSender := &fakePushSender{errFor: map[string]error{"token-1": errors.New("transient apns error")}}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	if err := uc.Execute(context.Background(), mobilePushEvent()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs.markExpired) != 0 {
		t.Fatalf("a generic send error must not mark the subscription expired, got %v", subs.markExpired)
	}
	if len(iosSender.sent) != 2 {
		t.Fatalf("expected both subscriptions attempted despite the first failing, got %d", len(iosSender.sent))
	}
}

func TestDeliverMobilePush_ListByUserErrorReturnsError(t *testing.T) {
	subs := &fakeSubscriptionRepository{listErr: errors.New("db unavailable")}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: &fakePushSender{}}, nil)

	if err := uc.Execute(context.Background(), mobilePushEvent()); err == nil {
		t.Fatal("expected a ListByUser failure to return an error, not be swallowed")
	}
}

// --- TASK-BE-MOBILE-008: HandleIncomingEvent wiring ---

func TestHandleIncomingEvent_PushChannelEventCallsDeliverMobilePush(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{iosTestSubscription(t, "s1", "ios-token-1")}
	iosSender := &fakePushSender{}
	deliverMobilePush := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, noopDeliverPush(), deliverMobilePush, nil)

	// orca.task.task.completed translates with both ws+push channels per
	// domain.TranslateEvent's subjectRules table.
	in := HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: timeNow(), Payload: []byte(`{"user_id":"user-1","title":"t","body":"b"}`),
	}
	if err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 1 || iosSender.sent[0] != "ios-token-1" {
		t.Fatalf("expected DeliverMobilePush to have sent to ios-token-1, got %v", iosSender.sent)
	}
}

func TestHandleIncomingEvent_WSOnlyEventDoesNotCallDeliverMobilePush(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{iosTestSubscription(t, "s1", "ios-token-1")}
	iosSender := &fakePushSender{}
	deliverMobilePush := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, noopDeliverPush(), deliverMobilePush, nil)

	// orca.tenant.star_nag.visibility_changed is WS-only per subjectRules
	// (Channels: []DeliveryChannel{ChannelDeliveryWS} — no push).
	in := HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.tenant.star_nag.visibility_changed",
		OccurredAt: timeNow(), Payload: []byte(`{"user_id":"user-1","body":"{}"}`),
	}
	if err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 0 {
		t.Fatalf("expected DeliverMobilePush NOT to be invoked for a WS-only subject, got %v", iosSender.sent)
	}
}

func TestHandleIncomingEvent_DeliverMobilePushErrorDoesNotFailExecute(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{iosTestSubscription(t, "s1", "ios-token-1")}
	iosSender := &fakePushSender{errFor: map[string]error{"ios-token-1": errors.New("apns down")}}
	deliverMobilePush := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, noopDeliverPush(), deliverMobilePush, nil)

	in := HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: timeNow(), Payload: []byte(`{"user_id":"user-1","title":"t","body":"b"}`),
	}
	if err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("a mobile push delivery failure must not fail HandleIncomingEvent.Execute: %v", err)
	}
	if len(iosSender.sent) != 1 {
		t.Fatalf("expected mobile push delivery to have been attempted, got %d sends", len(iosSender.sent))
	}
}

func TestDeliverMobilePush_MultipleRecipientsEachGetOwnSubscriptions(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		{ID: "s1", TenantID: "tenant-1", UserID: "user-1", Channel: domain.ChannelIOS, Endpoint: "token-user-1", Status: domain.SubscriptionActive},
		{ID: "s2", TenantID: "tenant-1", UserID: "user-2", Channel: domain.ChannelIOS, Endpoint: "token-user-2", Status: domain.SubscriptionActive},
	}
	iosSender := &fakePushSender{}
	uc := NewDeliverMobilePush(subs, map[domain.Channel]PushSender{domain.ChannelIOS: iosSender}, nil)

	event := domain.NotificationEvent{
		ID: "evt-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1", "user-2"}, Title: "t", Body: "b",
		Channels: []domain.DeliveryChannel{domain.ChannelDeliveryPush},
	}
	if err := uc.Execute(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iosSender.sent) != 2 {
		t.Fatalf("expected 1 send per recipient (2 total), got %d: %v", len(iosSender.sent), iosSender.sent)
	}
}
