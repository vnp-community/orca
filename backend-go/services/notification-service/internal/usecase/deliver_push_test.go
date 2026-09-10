package usecase

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func timeNow() time.Time { return time.Now() }

// alwaysSignOK returns a fakeVaultSigner func that signs whatever
// signing_input BuildVapidAuthHeader actually constructs, with a fresh
// real ECDSA P-256 key — for tests here that only care about "signing
// succeeds," not about verifying the resulting header (that's
// vapid_test.go's job).
func alwaysSignOK(t *testing.T) func(context.Context, string, []byte) (string, error) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate signer key: %v", err)
	}
	return func(_ context.Context, _ string, payload []byte) (string, error) {
		return vaultWireSignature(t, priv, payload), nil
	}
}

// fakeWebPushSender records every Send call and lets a test script
// per-endpoint outcomes (success / expired / transient error) — the same
// "in-memory double, no real network" pattern every other fake in this
// package follows.
type fakeWebPushSender struct {
	sendCalls int
	sent      []string // endpoints Send was called with, in call order

	// expiredEndpoints/errEndpoints let a test script per-endpoint
	// outcomes; an endpoint not listed in either succeeds (expired=false,
	// err=nil).
	expiredEndpoints map[string]bool
	errEndpoints     map[string]error
}

func (f *fakeWebPushSender) Send(ctx context.Context, sub domain.PushSubscription, vapidAuthHeader string, payload []byte) (bool, error) {
	f.sendCalls++
	f.sent = append(f.sent, sub.Endpoint)
	if f.expiredEndpoints[sub.Endpoint] {
		return true, nil
	}
	if err, ok := f.errEndpoints[sub.Endpoint]; ok {
		return false, err
	}
	return false, nil
}

// webPushTestSubscription builds a Channel==web PushSubscription with
// syntactically valid (if not cryptographically meaningful) p256dh/auth
// keys — DeliverPush never calls encrypt() itself (that's Sender's job,
// faked out here), so the key VALUES don't need to be real, only present
// (domain.NewPushSubscription requires non-nil for channel==web).
func webPushTestSubscription(t *testing.T, id, endpoint string) domain.PushSubscription {
	t.Helper()
	p256dh, auth := "p256dh-key", "auth-key"
	sub, err := domain.NewPushSubscription(id, "tenant-1", "user-1", domain.ChannelWeb, endpoint, &p256dh, &auth, "", timeNow())
	if err != nil {
		t.Fatalf("building web subscription: %v", err)
	}
	return sub
}

func iosTestSubscription(t *testing.T, id, endpoint string) domain.PushSubscription {
	t.Helper()
	sub, err := domain.NewPushSubscription(id, "tenant-1", "user-1", domain.ChannelIOS, endpoint, nil, nil, "", timeNow())
	if err != nil {
		t.Fatalf("building ios subscription: %v", err)
	}
	return sub
}

func pushEvent(channels ...domain.DeliveryChannel) domain.NotificationEvent {
	return domain.NotificationEvent{
		ID: "evt-1", TenantID: "tenant-1", RecipientUserIDs: []string{"user-1"},
		Type: "task.assigned", Title: "t", Body: "b", Channels: channels, CreatedAt: timeNow(),
	}
}

func TestDeliverPush_ChannelsWithoutPush_NoOp(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	sender := &fakeWebPushSender{}
	uc := NewDeliverPush(subs, keys, &fakeVaultSigner{}, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryWS)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sendCalls != 0 {
		t.Fatalf("expected 0 Send calls when Channels has no push, got %d", sender.sendCalls)
	}
}

func TestDeliverPush_SendsToEveryActiveWebSubscription(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		webPushTestSubscription(t, "s1", "https://push.example/ep-1"),
		webPushTestSubscription(t, "s2", "https://push.example/ep-2"),
		iosTestSubscription(t, "s3", "ios-device-token"),
	}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	signer := &fakeVaultSigner{fn: alwaysSignOK(t)}
	sender := &fakeWebPushSender{}
	uc := NewDeliverPush(subs, keys, signer, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryPush)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sendCalls != 2 {
		t.Fatalf("expected exactly 2 Send calls (web subscriptions only, not ios), got %d: %v", sender.sendCalls, sender.sent)
	}
}

func TestDeliverPush_NoActiveSubscription_NoError(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	sender := &fakeWebPushSender{}
	uc := NewDeliverPush(subs, keys, &fakeVaultSigner{fn: alwaysSignOK(t)}, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryPush)); err != nil {
		t.Fatalf("expected no error when the recipient has no subscriptions, got: %v", err)
	}
	if sender.sendCalls != 0 {
		t.Fatalf("expected 0 Send calls, got %d", sender.sendCalls)
	}
}

func TestDeliverPush_OneSubscriptionFails_OthersStillAttempted(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		webPushTestSubscription(t, "s1", "https://push.example/ep-1"),
		webPushTestSubscription(t, "s2", "https://push.example/ep-2"),
	}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	sender := &fakeWebPushSender{errEndpoints: map[string]error{"https://push.example/ep-1": errors.New("transient network error")}}
	uc := NewDeliverPush(subs, keys, &fakeVaultSigner{fn: alwaysSignOK(t)}, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryPush)); err != nil {
		t.Fatalf("a single subscription's send failure must not fail Execute: %v", err)
	}
	if sender.sendCalls != 2 {
		t.Fatalf("expected both subscriptions attempted despite the first failing, got %d calls: %v", sender.sendCalls, sender.sent)
	}
}

func TestDeliverPush_ExpiredSubscription_CallsMarkExpired(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	endpoint := "https://push.example/ep-1"
	subs.saved = []domain.PushSubscription{webPushTestSubscription(t, "s1", endpoint)}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	sender := &fakeWebPushSender{expiredEndpoints: map[string]bool{endpoint: true}}
	uc := NewDeliverPush(subs, keys, &fakeVaultSigner{fn: alwaysSignOK(t)}, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryPush)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs.markExpired) != 1 || subs.markExpired[0] != endpoint {
		t.Fatalf("expected MarkExpired called once with %q, got %v", endpoint, subs.markExpired)
	}
}

func TestDeliverPush_NoActiveVapidKey_NoOpNoError(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{webPushTestSubscription(t, "s1", "https://push.example/ep-1")}
	keys := &fakeVapidKeyRepository{err: domain.ErrNoActiveVapidKey}
	sender := &fakeWebPushSender{}
	uc := NewDeliverPush(subs, keys, &fakeVaultSigner{}, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryPush)); err != nil {
		t.Fatalf("no active vapid key must be a no-op, not an error: %v", err)
	}
	if sender.sendCalls != 0 {
		t.Fatalf("expected 0 Send calls when there is no vapid key to sign with, got %d", sender.sendCalls)
	}
}

func TestDeliverPush_SignerCalledOncePerSubscription(t *testing.T) {
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{
		webPushTestSubscription(t, "s1", "https://push.example/ep-1"),
		webPushTestSubscription(t, "s2", "https://push.example/ep-2"),
	}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	signer := &fakeVaultSigner{fn: alwaysSignOK(t)}
	sender := &fakeWebPushSender{}
	uc := NewDeliverPush(subs, keys, signer, sender, "mailto:ops@orca.dev", nil)

	if err := uc.Execute(context.Background(), pushEvent(domain.ChannelDeliveryPush)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signer.calls != 2 {
		t.Fatalf("expected SignVapidPayload called once per web subscription (2), got %d", signer.calls)
	}
}

func TestHandleIncomingEvent_DeliverPushErrorDoesNotFailExecute(t *testing.T) {
	// DeliverPush.Execute itself never returns a non-nil error for a
	// per-subscription send failure (see deliverOne's log-and-continue
	// design) — the only realistic non-nil-error path is the payload
	// marshal failure, which can't happen with a well-formed
	// domain.NotificationEvent. This test instead documents the wiring
	// contract at the HandleIncomingEvent level: even when push delivery
	// finds subscriptions and one of the underlying sends fails, Execute
	// as a whole must still return nil (the event is "handled" once WS
	// broadcast succeeds).
	subs := &fakeSubscriptionRepository{}
	subs.saved = []domain.PushSubscription{webPushTestSubscription(t, "s1", "https://push.example/ep-1")}
	keys := &fakeVapidKeyRepository{key: domain.VapidKeyMetadata{PublicKey: "pk"}}
	sender := &fakeWebPushSender{errEndpoints: map[string]error{"https://push.example/ep-1": errors.New("boom")}}
	deliverPush := NewDeliverPush(subs, keys, &fakeVaultSigner{fn: alwaysSignOK(t)}, sender, "mailto:ops@orca.dev", nil)

	b := &fakeBroadcaster{}
	uc := NewHandleIncomingEvent(b, &fakeProcessedEventRepository{}, &fakeNotificationRepository{}, deliverPush, noopDeliverMobilePush(), nil)

	// orca.task.task.completed translates with both ws+push channels per
	// domain.TranslateEvent's subjectRules table.
	in := HandleIncomingEventInput{
		EventID: "evt-1", TenantID: "tenant-1", Subject: "orca.task.task.completed",
		OccurredAt: timeNow(), Payload: []byte(`{"user_id":"user-1","title":"t","body":"b"}`),
	}
	if err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("a push delivery failure must not fail HandleIncomingEvent.Execute: %v", err)
	}
	if sender.sendCalls != 1 {
		t.Fatalf("expected push delivery to have been attempted, got %d Send calls", sender.sendCalls)
	}
}
