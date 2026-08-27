// Package usecase holds notification-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"

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
