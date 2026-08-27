package domain

import (
	"errors"
	"time"
)

// VapidKeyStatus tracks a tenant's VAPID keypair rotation state.
type VapidKeyStatus string

const (
	VapidKeyActive   VapidKeyStatus = "active"
	VapidKeyRotating VapidKeyStatus = "rotating"
	VapidKeyRevoked  VapidKeyStatus = "revoked"
)

// ErrNoActiveVapidKey is returned when a tenant has no active VAPID key row
// — GetVapidPublicKey has nothing to hand back to a client.
var ErrNoActiveVapidKey = errors.New("domain: no active vapid key for tenant")

// VapidKeyMetadata is the public half of a tenant's VAPID keypair plus a
// pointer to where the private half lives — see notification-service.md §4.
// VaultKeyRef is a Transit key *name*, not a value: reading this struct
// gives no ability to sign anything (§9). Invariant: exactly one Status ==
// VapidKeyActive row per tenant, enforced by a partial unique index (§5).
// The private key never appears as a field here, ever.
type VapidKeyMetadata struct {
	KeyID       string
	TenantID    string
	PublicKey   string
	VaultKeyRef string
	Status      VapidKeyStatus
	CreatedAt   time.Time
	RevokedAt   time.Time
}
