// Package domain holds notification-service's entities and value objects.
// Per specs/backend-go/architecture/03-clean-architecture-guidelines.md,
// this package has zero imports outside stdlib + other domain/ packages —
// no database, no gRPC, no NATS, no Vault client.
package domain

import (
	"errors"
	"time"
)

// Channel is the delivery mechanism a PushSubscription targets. Web Push
// (browser) carries RFC 8291 encryption keys; ios/android are device-token
// channels for APNs/FCM — see notification-service.md §4.
type Channel string

const (
	ChannelWeb     Channel = "web"
	ChannelIOS     Channel = "ios"
	ChannelAndroid Channel = "android"
)

// Valid reports whether c is one of the known enum values.
func (c Channel) Valid() bool {
	switch c {
	case ChannelWeb, ChannelIOS, ChannelAndroid:
		return true
	default:
		return false
	}
}

// SubscriptionStatus tracks a push subscription's lifecycle.
type SubscriptionStatus string

const (
	SubscriptionActive  SubscriptionStatus = "active"
	SubscriptionExpired SubscriptionStatus = "expired"
	SubscriptionRevoked SubscriptionStatus = "revoked"
)

// Domain errors — the taxonomy usecase/ maps to apperrors.Kind at the
// boundary (see architecture/03: "adapter layer is the only place a domain
// error gets mapped to a wire status code").
var (
	// ErrEmptyTenant is returned when TenantID/UserID are empty.
	ErrEmptyTenant = errors.New("domain: tenant_id and user_id are required")
	// ErrInvalidChannel is returned when Channel isn't a known enum value.
	ErrInvalidChannel = errors.New("domain: invalid push subscription channel")
	// ErrEmptyEndpoint is returned when Endpoint is empty.
	ErrEmptyEndpoint = errors.New("domain: endpoint is required")
	// ErrMissingWebKeys is returned when Channel == web but P256dhKey/AuthKey
	// are missing — both are required iff Channel == web (§4).
	ErrMissingWebKeys = errors.New("domain: p256dh_key and auth_key are required for the web channel")
	// ErrSubscriptionNotFound is a repository-level not-found sentinel.
	ErrSubscriptionNotFound = errors.New("domain: push subscription not found")
	// ErrUnsupportedChannel guards code paths that only handle a subset of
	// channels (e.g. VAPID Web Push signing, which is web-only).
	ErrUnsupportedChannel = errors.New("domain: unsupported delivery channel")
)

// PushSubscription is one browser/APNs/FCM endpoint a user registered to
// receive push notifications — see notification-service.md §4.
// P256dhKey/AuthKey are the browser's Web Push encryption keys (RFC 8291),
// a different key pair from VAPID entirely; user-identifying but not Vault
// material.
type PushSubscription struct {
	ID          string
	TenantID    string
	UserID      string
	Channel     Channel
	Endpoint    string
	P256dhKey   *string
	AuthKey     *string
	DeviceLabel string
	Status      SubscriptionStatus
	LastUsedAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewPushSubscription constructs a PushSubscription, enforcing the
// invariants a row must satisfy to be meaningful — mirrors
// usage-service.md's NewUsageSession pattern: this is where "this service
// owns this data's correctness" actually lives, not scattered validation
// in the gRPC handler.
func NewPushSubscription(
	id, tenantID, userID string,
	channel Channel,
	endpoint string,
	p256dhKey, authKey *string,
	deviceLabel string,
	now time.Time,
) (PushSubscription, error) {
	if tenantID == "" || userID == "" {
		return PushSubscription{}, ErrEmptyTenant
	}
	if !channel.Valid() {
		return PushSubscription{}, ErrInvalidChannel
	}
	if endpoint == "" {
		return PushSubscription{}, ErrEmptyEndpoint
	}
	if channel == ChannelWeb {
		if p256dhKey == nil || *p256dhKey == "" || authKey == nil || *authKey == "" {
			return PushSubscription{}, ErrMissingWebKeys
		}
	}
	return PushSubscription{
		ID:          id,
		TenantID:    tenantID,
		UserID:      userID,
		Channel:     channel,
		Endpoint:    endpoint,
		P256dhKey:   p256dhKey,
		AuthKey:     authKey,
		DeviceLabel: deviceLabel,
		Status:      SubscriptionActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
