package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// DeviceSecretResolver resolves a paired mobile device's shared secret
// (SOL-MB-01) via auth-service's internal-only ResolveDeviceSharedSecret
// RPC — implemented by internal/adapter/grpcclient/authclient.
type DeviceSecretResolver interface {
	ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error)
}

// E2ESealer NaCl-secretbox-encrypts a push payload with a paired device's
// shared secret (SOL-MB-01) — BR-MB-05: encrypted before it ever crosses
// the network. Implemented by internal/adapter/nacl.Sealer.
type E2ESealer interface {
	Seal(plaintext []byte, sharedSecret []byte) (ciphertext, nonce []byte, err error)
}

// WebPushClient sends an already-encrypted (or, for a non-paired
// subscription, about-to-be-encrypted per RFC 8291 by the implementation
// itself) payload to a Web Push endpoint. Implemented by
// internal/adapter/external/webpush.Client.
type WebPushClient interface {
	// vapidAuth is the complete Authorization header value (RFC 8292).
	Send(ctx context.Context, endpoint, p256dh, auth string, ciphertext, nonce []byte, vapidAuth string, opts WebPushOptions) error
}

// WebPushOptions carries RFC 8030 delivery hints (TTL and Urgency headers).
type WebPushOptions struct {
	TTLSeconds int
	Urgency    string
}

// VapidAuthorizer returns the RFC 8292 Authorization header value for a push
// to endpoint. The private key stays in Vault: implementations sign through
// VaultSigner (credential-broker-service), never locally.
type VapidAuthorizer interface {
	Authorization(ctx context.Context, tenantID, endpoint string) (string, error)
}

// PushObserver is the metrics hook. outcome is one of sent, expired, failed.
type PushObserver interface {
	ObserveDelivery(channel, outcome string, elapsed time.Duration)
}

type noopPushObserver struct{}

func (noopPushObserver) ObserveDelivery(string, string, time.Duration) {}

// DeliverPushConfig bounds fan-out work per event.
type DeliverPushConfig struct {
	// Concurrency caps simultaneous subscription deliveries (default 8).
	Concurrency int
	// Timeout is the deadline for each outbound delivery (default 5s).
	Timeout  time.Duration
	Observer PushObserver
}

// APNsClient sends an E2E-sealed push to Apple's APNs gateway — own
// credential (ES256 provider JWT), never VAPID. Implemented by
// internal/adapter/external/apns.Client.
type APNsClient interface {
	Send(ctx context.Context, deviceToken string, ciphertext, nonce []byte) error
}

// FCMClient sends an E2E-sealed push to Firebase Cloud Messaging — own
// credential (OAuth2 service-account token), never VAPID. Implemented by
// internal/adapter/external/fcm.Client.
type FCMClient interface {
	Send(ctx context.Context, registrationToken string, ciphertext, nonce []byte) error
}

// DeliverPush is notification-service's mobile/web push delivery
// path (notification-service.md §6's deliver_push.go) — the usecase that
// finally calls VaultSigner (previously wired but never invoked, per
// adapter/grpc.Server's old doc comment) for the web channel, and APNs/FCM
// (their own credential class, TASK-MB-02-08) for iOS/Android.
type DeliverPush struct {
	subscriptions SubscriptionRepository
	devices       DeviceSecretResolver
	sealer        E2ESealer
	vapidAuth     VapidAuthorizer
	webpush       WebPushClient
	buffer        BufferedNotificationRepository
	preferences   NotificationPreferenceRepository
	apns          APNsClient // nil when APNs is not configured (no Vault client / credentials) — deliverOne degrades to a clear error for that channel
	fcm           FCMClient  // nil when FCM is not configured — same degrade
	logger        *slog.Logger
	concurrency   int
	timeout       time.Duration
	observer      PushObserver
}

func NewDeliverPush(
	subs SubscriptionRepository,
	devices DeviceSecretResolver,
	sealer E2ESealer,
	vapidAuth VapidAuthorizer,
	webpush WebPushClient,
	buffer BufferedNotificationRepository,
	preferences NotificationPreferenceRepository,
	apns APNsClient,
	fcm FCMClient,
	logger *slog.Logger,
) *DeliverPush {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeliverPush{
		subscriptions: subs, devices: devices, sealer: sealer, vapidAuth: vapidAuth,
		webpush: webpush, buffer: buffer, preferences: preferences, apns: apns, fcm: fcm, logger: logger,
		concurrency: 8, timeout: 5 * time.Second, observer: noopPushObserver{},
	}
}

// Configure overrides the fan-out limits; zero values keep the defaults.
func (uc *DeliverPush) Configure(cfg DeliverPushConfig) *DeliverPush {
	if cfg.Concurrency > 0 {
		uc.concurrency = cfg.Concurrency
	}
	if cfg.Timeout > 0 {
		uc.timeout = cfg.Timeout
	}
	if cfg.Observer != nil {
		uc.observer = cfg.Observer
	}
	return uc
}

// Execute delivers event to every recipient's push subscriptions, one at a
// time, best-effort: a per-subscription failure buffers the event for that
// subscription (BR-MB-07) rather than failing the whole call — this
// usecase never returns an error itself, matching HandleIncomingEvent's
// "a push-delivery hiccup must not NAK the whole event" posture.
func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	hasPush := false
	for _, ch := range event.Channels {
		if ch == domain.ChannelDeliveryPush {
			hasPush = true
		}
	}
	if !hasPush {
		return nil
	}
	type job struct {
		userID string
		sub    domain.PushSubscription
	}
	var jobs []job
	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			uc.logger.WarnContext(ctx, "deliver_push: failed to list subscriptions", slog.String("user_id", userID), slog.Any("error", err))
			continue
		}
		for _, sub := range subs {
			allowed, err := uc.preferences.IsEnabled(ctx, event.TenantID, userID, event.Type, string(sub.Channel))
			if err != nil || !allowed { // BR-MB-08
				continue
			}
			jobs = append(jobs, job{userID, sub})
		}
	}

	// Why bounded fan-out: one slow push service must neither stall the
	// others nor let a large recipient list spawn unbounded goroutines; each
	// subscription is independent, so a failure never aborts the rest.
	sem := make(chan struct{}, uc.concurrency)
	var wg sync.WaitGroup
	for _, j := range jobs {
		sem <- struct{}{}
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			uc.deliverAndBuffer(ctx, event, j.userID, j.sub)
		}(j)
	}
	wg.Wait()
	return nil
}

func (uc *DeliverPush) deliverAndBuffer(ctx context.Context, event domain.NotificationEvent, userID string, sub domain.PushSubscription) {
	start := time.Now()
	sendCtx, cancel := context.WithTimeout(ctx, uc.timeout)
	err := uc.deliverOne(sendCtx, event, sub)
	cancel()

	switch {
	case err == nil:
		uc.observer.ObserveDelivery(string(sub.Channel), "sent", time.Since(start))
		return
	case errors.Is(err, ErrDeviceTokenInvalid):
		uc.observer.ObserveDelivery(string(sub.Channel), "expired", time.Since(start))
	default:
		uc.observer.ObserveDelivery(string(sub.Channel), "failed", time.Since(start))
	}
	uc.logger.WarnContext(ctx, "deliver_push: delivery failed",
		slog.String("subscription_id", sub.ID), slog.String("channel", string(sub.Channel)),
		slog.String("endpoint_host", endpointHost(sub.Endpoint)), slog.Any("error", err))
	// Buffered even for a dead endpoint (pre-existing BR-MB-07 behavior): the
	// buffer is the user's offline replay, and the endpoint is not retried.
	if bufErr := uc.buffer.Enqueue(ctx, event.TenantID, userID, sub.ID, mustJSON(event)); bufErr != nil { // BR-MB-07
		uc.logger.WarnContext(ctx, "deliver_push: failed to buffer undelivered notification",
			slog.String("subscription_id", sub.ID), slog.Any("deliver_error", err), slog.Any("buffer_error", bufErr))
	}
}

// endpointHost keeps only the push service's host for logs — the path/query
// of a Web Push endpoint is a bearer capability and must not be logged.
func endpointHost(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "unknown"
	}
	return u.Host
}

// deliverOne dispatches on sub.Channel. web/no-paired-device uses standard
// (non-E2E) Web Push, VAPID-signed; every other case (web+paired-device,
// ios, android) NaCl-seals the payload with the paired device's shared
// secret first (BR-MB-05), then routes to the channel's own transport —
// APNs/FCM use their OWN Transit-mediated credential (TASK-MB-02-08),
// never vapidSigner, which is web-only.
func (uc *DeliverPush) deliverOne(ctx context.Context, event domain.NotificationEvent, sub domain.PushSubscription) error {
	plaintext := mustJSON(event)
	delivery := domain.PushDeliveryFor(event)
	opts := WebPushOptions{TTLSeconds: delivery.TTLSeconds, Urgency: delivery.Urgency}

	deviceID, err := uc.subscriptions.DeviceIDFor(ctx, sub.ID)
	if err != nil || deviceID == "" {
		if sub.Channel != domain.ChannelWeb {
			// iOS/Android always require a paired device — no non-E2E
			// fallback for native push (TASK-MB-02-08).
			return domain.ErrUnsupportedChannel
		}
		// Standard Web Push (RFC 8291) encryption only, no BL-MB-01 E2E
		// layer — a web-channel subscription with no paired device is not
		// a mobile-companion flow.
		// The browser's service worker expects exactly
		// {title, body, deepLink, tag}, not the internal event shape.
		browserPayload, err := json.Marshal(domain.ToPushMessage(event))
		if err != nil {
			return err
		}
		auth, err := uc.vapidAuth.Authorization(ctx, event.TenantID, sub.Endpoint)
		if err != nil {
			return err
		}
		sendErr := uc.webpush.Send(ctx, sub.Endpoint, derefOrEmpty(sub.P256dhKey), derefOrEmpty(sub.AuthKey), browserPayload, nil, auth, opts)
		return uc.markExpiredOnDeadToken(ctx, sub.Endpoint, sendErr)
	}

	secret, err := uc.devices.ResolveSharedSecret(ctx, deviceID)
	if err != nil {
		return err
	}
	ciphertext, nonce, err := uc.sealer.Seal(plaintext, secret)
	if err != nil {
		return err
	}

	switch sub.Channel {
	case domain.ChannelWeb:
		auth, err := uc.vapidAuth.Authorization(ctx, event.TenantID, sub.Endpoint)
		if err != nil {
			return err
		}
		sendErr := uc.webpush.Send(ctx, sub.Endpoint, derefOrEmpty(sub.P256dhKey), derefOrEmpty(sub.AuthKey), ciphertext, nonce, auth, opts)
		return uc.markExpiredOnDeadToken(ctx, sub.Endpoint, sendErr)
	case domain.ChannelIOS:
		if uc.apns == nil {
			return errAPNsNotConfigured
		}
		sendErr := uc.apns.Send(ctx, sub.Endpoint, ciphertext, nonce) // own APNs credential — NOT vapidSigner
		return uc.markExpiredOnDeadToken(ctx, sub.Endpoint, sendErr)
	case domain.ChannelAndroid:
		if uc.fcm == nil {
			return errFCMNotConfigured
		}
		sendErr := uc.fcm.Send(ctx, sub.Endpoint, ciphertext, nonce) // own FCM credential — NOT vapidSigner
		return uc.markExpiredOnDeadToken(ctx, sub.Endpoint, sendErr)
	default:
		return domain.ErrUnsupportedChannel
	}
}

// markExpiredOnDeadToken inspects sendErr for ErrDeviceTokenInvalid — a
// third-party push service reporting the device token/endpoint as
// permanently dead (APNs BadDeviceToken/Unregistered, FCM UNREGISTERED, Web
// Push 404/410) — and, if found, marks the subscription expired
// (CR-MOBILE-001) before returning sendErr unchanged so the caller still
// sees (and buffers/logs) the original failure. A MarkExpired failure is
// logged, not propagated: a repository hiccup here must not mask or replace
// the real delivery error.
func (uc *DeliverPush) markExpiredOnDeadToken(ctx context.Context, endpoint string, sendErr error) error {
	if sendErr == nil {
		return nil
	}
	if errors.Is(sendErr, ErrDeviceTokenInvalid) {
		if err := uc.subscriptions.MarkExpired(ctx, endpoint); err != nil {
			uc.logger.WarnContext(ctx, "deliver_push: failed to mark expired subscription",
				slog.String("endpoint_host", endpointHost(endpoint)), slog.Any("error", err))
		}
	}
	return sendErr
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// mustJSON marshals event for wire framing / buffering. NotificationEvent
// is a plain data struct — marshal failure would indicate a bug in this
// function, not bad input — so it degrades to an empty object rather than
// panicking, mirroring adapter/grpc/frame.go's framePayloadJSON (a small,
// harmless duplication of shape rather than code, since usecase/ cannot
// import adapter/grpc across the Clean Architecture layer boundary).
func mustJSON(event domain.NotificationEvent) []byte {
	b, err := json.Marshal(event)
	if err != nil {
		return []byte("{}")
	}
	return b
}

var (
	errAPNsNotConfigured = deliverPushConfigError("notification: apns client not configured (no Vault client / APNS_* env vars) — cannot deliver to ios channel")
	errFCMNotConfigured  = deliverPushConfigError("notification: fcm client not configured (no Vault client / FCM_* env vars) — cannot deliver to android channel")
)

type deliverPushConfigError string

func (e deliverPushConfigError) Error() string { return string(e) }
