package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// --- fakes -------------------------------------------------------------

type fakeDeliverPushSubscriptionRepository struct {
	subs      []domain.PushSubscription
	deviceIDs map[string]string // subscriptionID -> deviceID
}

func (f *fakeDeliverPushSubscriptionRepository) Save(ctx context.Context, sub domain.PushSubscription) error {
	return nil
}
func (f *fakeDeliverPushSubscriptionRepository) ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error) {
	var out []domain.PushSubscription
	for _, s := range f.subs {
		if s.TenantID == tenantID && s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeDeliverPushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	return nil
}
func (f *fakeDeliverPushSubscriptionRepository) DeviceIDFor(ctx context.Context, subscriptionID string) (string, error) {
	if f.deviceIDs == nil {
		return "", nil
	}
	return f.deviceIDs[subscriptionID], nil
}

type fakeDeviceSecretResolver struct {
	secret []byte
	err    error
	calls  int
}

func (f *fakeDeviceSecretResolver) ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.secret, nil
}

type fakeE2ESealer struct {
	calls        int
	sealErr      error
	lastSecretLn int
}

func (f *fakeE2ESealer) Seal(plaintext []byte, sharedSecret []byte) ([]byte, []byte, error) {
	f.calls++
	f.lastSecretLn = len(sharedSecret)
	if f.sealErr != nil {
		return nil, nil, f.sealErr
	}
	return append([]byte("sealed:"), plaintext...), []byte("nonce-bytes-000000000000"), nil
}

type fakeVaultSigner struct {
	calls int
	err   error
}

func (f *fakeVaultSigner) SignVapidPayload(ctx context.Context, tenantID string, payload []byte) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return "signed-jwt", nil
}

type webpushCall struct {
	endpoint          string
	p256dh, auth      string
	ciphertext, nonce []byte
	vapidJWT          string
}

type fakeWebPushClient struct {
	calls []webpushCall
	err   error
}

func (f *fakeWebPushClient) Send(ctx context.Context, endpoint, p256dh, auth string, ciphertext, nonce []byte, vapidJWT string) error {
	f.calls = append(f.calls, webpushCall{endpoint, p256dh, auth, ciphertext, nonce, vapidJWT})
	return f.err
}

type fakeAPNsClient struct {
	calls int
	err   error
}

func (f *fakeAPNsClient) Send(ctx context.Context, deviceToken string, ciphertext, nonce []byte) error {
	f.calls++
	return f.err
}

type fakeFCMClient struct {
	calls int
	err   error
}

func (f *fakeFCMClient) Send(ctx context.Context, registrationToken string, ciphertext, nonce []byte) error {
	f.calls++
	return f.err
}

type fakeBufferedNotificationRepository struct {
	enqueued []string // subscriptionIDs
}

func (f *fakeBufferedNotificationRepository) Enqueue(ctx context.Context, tenantID, userID, subscriptionID string, eventJSON []byte) error {
	f.enqueued = append(f.enqueued, subscriptionID)
	return nil
}
func (f *fakeBufferedNotificationRepository) ListPending(ctx context.Context, tenantID, userID string) ([]domain.BufferedNotification, error) {
	return nil, nil
}
func (f *fakeBufferedNotificationRepository) MarkDelivered(ctx context.Context, ids []string) error {
	return nil
}

type fakeNotificationPreferenceRepository struct {
	disabled map[string]bool // "eventType|channel" -> disabled
}

func (f *fakeNotificationPreferenceRepository) IsEnabled(ctx context.Context, tenantID, userID, eventType, channel string) (bool, error) {
	if f.disabled != nil && f.disabled[eventType+"|"+channel] {
		return false, nil
	}
	return true, nil
}

func testEvent(tenantID string, userIDs ...string) domain.NotificationEvent {
	return domain.NotificationEvent{
		ID:               "ne-1",
		TenantID:         tenantID,
		RecipientUserIDs: userIDs,
		Type:             "task_completed",
		Title:            "Task completed",
		Channels:         []domain.DeliveryChannel{domain.ChannelDeliveryWS, domain.ChannelDeliveryPush},
		CreatedAt:        time.Now(),
	}
}

// --- tests ---------------------------------------------------------------

func TestDeliverPush_WebChannelNoPairedDevice_SignsAndSendsWithoutSealing(t *testing.T) {
	p256, auth := "p256dh-key", "auth-key"
	sub := domain.PushSubscription{ID: "sub-1", TenantID: "t1", UserID: "u1", Channel: domain.ChannelWeb, Endpoint: "https://push.example/1", P256dhKey: &p256, AuthKey: &auth}
	subs := &fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{sub}}
	sealer := &fakeE2ESealer{}
	signer := &fakeVaultSigner{}
	webpush := &fakeWebPushClient{}
	buffer := &fakeBufferedNotificationRepository{}
	prefs := &fakeNotificationPreferenceRepository{}

	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, sealer, signer, webpush, buffer, prefs, nil, nil, nil)

	if err := uc.Execute(context.Background(), testEvent("t1", "u1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sealer.calls != 0 {
		t.Errorf("expected sealer.Seal NOT called for no-paired-device web subscription, got %d calls", sealer.calls)
	}
	if signer.calls != 1 {
		t.Errorf("expected vapidSigner called once, got %d", signer.calls)
	}
	if len(webpush.calls) != 1 {
		t.Fatalf("expected webpush.Send called once, got %d", len(webpush.calls))
	}
	if len(buffer.enqueued) != 0 {
		t.Errorf("expected no buffering on success, got %v", buffer.enqueued)
	}
}

func TestDeliverPush_WebChannelPairedDevice_SealsThenSignsCiphertext(t *testing.T) {
	p256, auth := "p256dh-key", "auth-key"
	sub := domain.PushSubscription{ID: "sub-1", TenantID: "t1", UserID: "u1", Channel: domain.ChannelWeb, Endpoint: "https://push.example/1", P256dhKey: &p256, AuthKey: &auth}
	subs := &fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{sub}, deviceIDs: map[string]string{"sub-1": "device-1"}}
	sealer := &fakeE2ESealer{}
	signer := &fakeVaultSigner{}
	webpush := &fakeWebPushClient{}
	devices := &fakeDeviceSecretResolver{secret: make([]byte, 32)}

	uc := NewDeliverPush(subs, devices, sealer, signer, webpush, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, nil, nil, nil)

	if err := uc.Execute(context.Background(), testEvent("t1", "u1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sealer.calls != 1 {
		t.Errorf("expected sealer.Seal called once for paired-device web subscription, got %d", sealer.calls)
	}
	if len(webpush.calls) != 1 {
		t.Fatalf("expected webpush.Send called once, got %d", len(webpush.calls))
	}
	call := webpush.calls[0]
	if call.ciphertext == nil || call.nonce == nil {
		t.Errorf("expected non-nil ciphertext/nonce sent to webpush.Send, got ciphertext=%v nonce=%v", call.ciphertext, call.nonce)
	}
	if string(call.ciphertext) == `{"...":"..."}` {
		t.Errorf("webpush.Send must never receive the plaintext event body")
	}
}

func TestDeliverPush_DeliveryFailure_BuffersOnce(t *testing.T) {
	p256, auth := "p256dh-key", "auth-key"
	sub := domain.PushSubscription{ID: "sub-1", TenantID: "t1", UserID: "u1", Channel: domain.ChannelWeb, Endpoint: "https://push.example/1", P256dhKey: &p256, AuthKey: &auth}
	subs := &fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{sub}}
	webpush := &fakeWebPushClient{err: errors.New("endpoint unreachable")}
	buffer := &fakeBufferedNotificationRepository{}

	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, webpush, buffer, &fakeNotificationPreferenceRepository{}, nil, nil, nil)

	if err := uc.Execute(context.Background(), testEvent("t1", "u1")); err != nil {
		t.Fatalf("Execute itself must not return an error: %v", err)
	}
	if len(buffer.enqueued) != 1 {
		t.Errorf("expected buffer.Enqueue called once on delivery failure, got %d", len(buffer.enqueued))
	}
}

func TestDeliverPush_DisabledPreference_SkipsDelivery(t *testing.T) {
	p256, auth := "p256dh-key", "auth-key"
	sub := domain.PushSubscription{ID: "sub-1", TenantID: "t1", UserID: "u1", Channel: domain.ChannelWeb, Endpoint: "https://push.example/1", P256dhKey: &p256, AuthKey: &auth}
	subs := &fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{sub}}
	webpush := &fakeWebPushClient{}
	prefs := &fakeNotificationPreferenceRepository{disabled: map[string]bool{"task_completed|web": true}}

	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, webpush, &fakeBufferedNotificationRepository{}, prefs, nil, nil, nil)

	if err := uc.Execute(context.Background(), testEvent("t1", "u1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(webpush.calls) != 0 {
		t.Errorf("expected deliverOne never invoked for a disabled preference, got %d webpush calls", len(webpush.calls))
	}
}

func TestDeliverPush_IOSAndAndroid_UseOwnCredential_NeverVAPID(t *testing.T) {
	sub := func(id string, ch domain.Channel) domain.PushSubscription {
		return domain.PushSubscription{ID: id, TenantID: "t1", UserID: "u1", Channel: ch, Endpoint: "token-" + id}
	}
	subs := &fakeDeliverPushSubscriptionRepository{
		subs:      []domain.PushSubscription{sub("sub-ios", domain.ChannelIOS), sub("sub-android", domain.ChannelAndroid)},
		deviceIDs: map[string]string{"sub-ios": "device-ios", "sub-android": "device-android"},
	}
	signer := &fakeVaultSigner{}
	apns := &fakeAPNsClient{}
	fcm := &fakeFCMClient{}
	devices := &fakeDeviceSecretResolver{secret: make([]byte, 32)}

	uc := NewDeliverPush(subs, devices, &fakeE2ESealer{}, signer, &fakeWebPushClient{}, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, apns, fcm, nil)

	if err := uc.Execute(context.Background(), testEvent("t1", "u1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apns.calls != 1 {
		t.Errorf("expected apns.Send called once, got %d", apns.calls)
	}
	if fcm.calls != 1 {
		t.Errorf("expected fcm.Send called once, got %d", fcm.calls)
	}
	if signer.calls != 0 {
		t.Errorf("expected vapidSigner.SignVapidPayload NEVER called for ios/android channels (VAPID/APNs conflation regression guard), got %d calls", signer.calls)
	}
}

func TestDeliverPush_NoPushChannel_IsNoop(t *testing.T) {
	subs := &fakeDeliverPushSubscriptionRepository{}
	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, &fakeWebPushClient{}, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, nil, nil, nil)

	event := testEvent("t1", "u1")
	event.Channels = []domain.DeliveryChannel{domain.ChannelDeliveryWS}
	if err := uc.Execute(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
