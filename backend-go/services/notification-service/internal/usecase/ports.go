// Package usecase holds notification-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// SubscriptionRepository is the persistence port for push subscriptions.
// Implemented by internal/adapter/postgres against this service's own
// database — see architecture/05-data-architecture.md's
// database-per-service rule.
type SubscriptionRepository interface {
	// Save upserts a subscription (re-subscribing to the same endpoint
	// reactivates/updates it rather than erroring — see the postgres
	// adapter's ON CONFLICT(endpoint) clause).
	Save(ctx context.Context, sub domain.PushSubscription) error
	// ListByUser returns a tenant's user's active subscriptions.
	ListByUser(ctx context.Context, tenantID, userID string) ([]domain.PushSubscription, error)
	// DeleteByEndpoint removes the subscription row for endpoint (matches
	// push_subscriptions.endpoint's unique index). Deleting an endpoint
	// with no matching row affects 0 rows and is NOT an error — the
	// unregister operation is idempotent by design.
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	// MarkExpired sets status='expired' for endpoint — called when a Web
	// Push send returns 404/410, so a future event doesn't retry a dead
	// endpoint. Idempotent: marking an already-expired/nonexistent endpoint
	// affects 0 rows and is NOT an error, same rule as DeleteByEndpoint.
	MarkExpired(ctx context.Context, endpoint string) error
}

// WebPushSender sends one already-encrypted Web Push message to one
// subscription's endpoint. Implemented by internal/adapter/external/webpush
// (TASK-BE-NOTIF-010) — this port only knows "send bytes, get back whether
// the endpoint is dead", not the RFC 8291/8292 mechanics.
type WebPushSender interface {
	// Send POSTs payload (already RFC-8291-encrypted) to sub.Endpoint with
	// vapidAuthHeader as the Authorization header. expired=true means the
	// push service returned 404/410 — the endpoint is gone, the caller
	// must not retry it and should call SubscriptionRepository.MarkExpired.
	// Any other non-2xx status or transport error is returned as err
	// (expired=false) — a transient failure, not "this subscription is dead".
	Send(ctx context.Context, sub domain.PushSubscription, vapidAuthHeader string, payload []byte) (expired bool, err error)
}

// VapidKeyRepository is the persistence port for VAPID public-key
// metadata. Never exposes the private key — that never enters this
// process at all (§9); the repository only ever reads/writes the public
// half and a Vault Transit key-name pointer.
type VapidKeyRepository interface {
	GetPublicKey(ctx context.Context, tenantID string) (domain.VapidKeyMetadata, error)
}

// VaultSigner signs a VAPID push payload via Vault's Transit engine —
// notification-service.md §9's headline property: "signing a push
// payload's VAPID JWT is a Vault: sign call, not read secret, then sign
// locally." Implemented by internal/adapter/vaultsigner, which wraps
// common/secrets.Client.TransitEncrypt (the Transit-engine primitive
// common/secrets exposes today) — there is no adapter/vault/ package here
// and no other path to VAPID signing, matching the "this service never
// calls Vault directly for anything except the Transit path" rule note in
// this service's README.
type VaultSigner interface {
	SignVapidPayload(ctx context.Context, tenantID string, payload []byte) (string, error)
}

// ProcessedEventRepository is the persistence port for JetStream
// consumer-side dedup — notification-service.md §5/§8's
// notification.processed_events table, guarding HandleIncomingEvent against
// at-least-once redelivery of the same event ID.
type ProcessedEventRepository interface {
	// MarkProcessed atomically reserves eventID: the first caller for a
	// given eventID gets alreadyProcessed=false and should proceed with
	// translation/broadcast; every later caller (a JetStream redelivery, or
	// another replica racing the same redelivered message — SubscribeEphemeral
	// gives every notification-service replica its own independent
	// consumer, so this is a real concurrent scenario, not just
	// single-process retry) gets alreadyProcessed=true and must skip
	// re-processing. Implemented as a single atomic
	// INSERT ... ON CONFLICT DO NOTHING, not a racy check-then-insert.
	MarkProcessed(ctx context.Context, eventID, subject string) (alreadyProcessed bool, err error)
}

// NotificationBroadcaster fans a translated NotificationEvent out to
// active StreamNotifications subscribers, keyed by tenant+user. A real,
// working in-process implementation lives in internal/adapter/broadcaster.
// It is still replica-local (Broadcast only reaches subscribers connected
// to the same process) — cross-replica delivery is handled one layer up,
// by internal/adapter/eventbus's Consumer giving every replica its own
// independent subscription to each domain event, so every replica calls
// Broadcast for every event (docs/execution-plan.md Epic F).
type NotificationBroadcaster interface {
	// Subscribe registers a channel for tenantID+userID and returns it
	// plus an unsubscribe func the caller MUST invoke exactly once when
	// the stream ends (e.g. via defer), or the registry leaks the
	// subscriber entry.
	Subscribe(ctx context.Context, tenantID, userID string) (<-chan domain.NotificationEvent, func())
	// Broadcast delivers event to every currently-subscribed channel for
	// each of event.RecipientUserIDs. A recipient with no active
	// subscription simply doesn't receive it — no offline WS replay
	// queue, per notification-service.md §2.
	Broadcast(ctx context.Context, event domain.NotificationEvent)
}

// NotificationRepository is the persistence port for notification_events —
// the audit/unread-state store CR-NOTIF-001 adds. Every method takes
// tenantID + userID explicitly (never trusts a bare notificationID) so a
// caller cannot mark-as-read or read another tenant's/user's row by
// guessing an ID — see architecture/05's tenant-isolation rule.
type NotificationRepository interface {
	// SaveNotificationEvent persists 1 row per event.RecipientUserIDs entry.
	// Called from HandleIncomingEvent BEFORE Broadcast (TASK-BE-NOTIF-003)
	// so a crash between the two leaves a persisted record, not a lost one.
	// Named SaveNotificationEvent (not the task doc's plain "Save") because
	// the same *postgres.Repository also implements SubscriptionRepository,
	// whose Save(ctx, domain.PushSubscription) already claims that name on
	// the shared receiver — Go has no signature-based overloading.
	SaveNotificationEvent(ctx context.Context, event domain.NotificationEvent) error
	// ListByRecipient returns cursor-paginated notifications for
	// tenantID+userID, newest first. cursor is opaque (empty string means
	// "first page"); returned nextCursor is empty when there is no next
	// page. unreadOnly filters to IsRead == false.
	ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error)
	// MarkAsRead sets is_read=true, read_at=now() for notificationID
	// scoped to tenantID+userID. Idempotent: marking an already-read row
	// again is a successful no-op, not an error (mirrors
	// DeleteByEndpoint's idempotency rule).
	MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error
	// MarkAllAsRead sets is_read=true, read_at=now() for every unread row
	// of tenantID+userID; returns the number of rows updated (0 is not an
	// error — nothing to mark).
	MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error)
	// CountUnread returns the count of is_read=false rows for
	// tenantID+userID.
	CountUnread(ctx context.Context, tenantID, userID string) (int64, error)
}

// PushCredentialResolver fetches this tenant's APNs/FCM credential
// material via credential-broker-service (CREDENTIAL_CATEGORY_SERVICE_SECRET
// — already defined in credentialbroker.proto and handled by that
// service's grpc/server.go, but exercised by no caller before this).
// Distinct from VaultSigner: that port signs a VAPID JWT for Web Push;
// this one returns raw credential bytes for a different protocol
// entirely (APNs JWT ES256 / FCM OAuth2 service-account) — see
// notification-service.md §9's updated credential-storage section.
type PushCredentialResolver interface {
	// Resolve returns the opaque credential blob stored for
	// (tenantID, ownerID) — ownerID is "apns" or "fcm". The blob's shape
	// is adapter-owned (see internal/adapter/pushgateway): apns_sender.go
	// JSON-decodes it as {team_id, key_id, private_key_pem}; fcm_sender.go
	// treats it as the raw FCM service-account JSON as-is.
	Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error)
}

// PushSender delivers event to one ios/android push subscription — the
// sole difference between APNsSender and FCMSender is which third-party
// wire protocol they speak; DeliverPush (mobile companion, CR-MOBILE-001)
// picks one by subscription.Channel via a map, not a type switch.
type PushSender interface {
	Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error
}

// ErrDeviceTokenInvalid is returned by PushSender.Send when the
// third-party service reports the device token/endpoint no longer
// accepts pushes (APNs 410/BadDeviceToken, FCM 404/UNREGISTERED).
// DeliverPush treats this as "call MarkExpired", never a generic error.
var ErrDeviceTokenInvalid = errors.New("usecase: device token no longer valid")
