package usecase

import (
	"context"
	"encoding/json"
	"log/slog"

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
	Send(ctx context.Context, endpoint, p256dh, auth string, ciphertext, nonce []byte, vapidJWT string) error
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
	vapidSigner   VaultSigner
	webpush       WebPushClient
	buffer        BufferedNotificationRepository
	preferences   NotificationPreferenceRepository
	apns          APNsClient // nil when APNs is not configured (no Vault client / credentials) — deliverOne degrades to a clear error for that channel
	fcm           FCMClient  // nil when FCM is not configured — same degrade
	logger        *slog.Logger
}

func NewDeliverPush(
	subs SubscriptionRepository,
	devices DeviceSecretResolver,
	sealer E2ESealer,
	vapidSigner VaultSigner,
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
		subscriptions: subs, devices: devices, sealer: sealer, vapidSigner: vapidSigner,
		webpush: webpush, buffer: buffer, preferences: preferences, apns: apns, fcm: fcm, logger: logger,
	}
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
			if err := uc.deliverOne(ctx, event, sub); err != nil {
				payloadJSON := mustJSON(event)
				if bufErr := uc.buffer.Enqueue(ctx, event.TenantID, userID, sub.ID, payloadJSON); bufErr != nil { // BR-MB-07
					uc.logger.WarnContext(ctx, "deliver_push: failed to buffer undelivered notification",
						slog.String("subscription_id", sub.ID), slog.Any("deliver_error", err), slog.Any("buffer_error", bufErr))
				}
			}
		}
	}
	return nil
}

// deliverOne dispatches on sub.Channel. web/no-paired-device uses standard
// (non-E2E) Web Push, VAPID-signed; every other case (web+paired-device,
// ios, android) NaCl-seals the payload with the paired device's shared
// secret first (BR-MB-05), then routes to the channel's own transport —
// APNs/FCM use their OWN Transit-mediated credential (TASK-MB-02-08),
// never vapidSigner, which is web-only.
func (uc *DeliverPush) deliverOne(ctx context.Context, event domain.NotificationEvent, sub domain.PushSubscription) error {
	plaintext := mustJSON(event)

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
		jwt, err := uc.vapidSigner.SignVapidPayload(ctx, event.TenantID, plaintext)
		if err != nil {
			return err
		}
		return uc.webpush.Send(ctx, sub.Endpoint, derefOrEmpty(sub.P256dhKey), derefOrEmpty(sub.AuthKey), plaintext, nil, jwt)
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
		jwt, err := uc.vapidSigner.SignVapidPayload(ctx, event.TenantID, ciphertext)
		if err != nil {
			return err
		}
		return uc.webpush.Send(ctx, sub.Endpoint, derefOrEmpty(sub.P256dhKey), derefOrEmpty(sub.AuthKey), ciphertext, nonce, jwt)
	case domain.ChannelIOS:
		if uc.apns == nil {
			return errAPNsNotConfigured
		}
		return uc.apns.Send(ctx, sub.Endpoint, ciphertext, nonce) // own APNs credential — NOT vapidSigner
	case domain.ChannelAndroid:
		if uc.fcm == nil {
			return errFCMNotConfigured
		}
		return uc.fcm.Send(ctx, sub.Endpoint, ciphertext, nonce) // own FCM credential — NOT vapidSigner
	default:
		return domain.ErrUnsupportedChannel
	}
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
