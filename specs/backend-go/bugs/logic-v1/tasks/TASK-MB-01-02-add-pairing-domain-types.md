# TASK-MB-01-02: Add `PairingSession`/`PairedDevice` domain types

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `auth-service`
**File:** `backend-go/services/auth-service/internal/domain/pairing_session.go`, `backend-go/services/auth-service/internal/domain/paired_device.go`
**Depends on:** none
**Status:** [x] DONE — `PairingSession`/`PairedDevice` domain types added with BR-MB-01/02/03 invariants; `go build`/`go vet ./services/auth-service/...` clean.

---

## Context

BR-MB-01 (5-minute pairing-token expiry) and BR-MB-02 (one-time use) must
be invariants of the domain type itself, not scattered `if` checks in the
usecase layer — matching this codebase's existing constructor-validation
convention (e.g. `project-service`'s `NewWorktree`).

## Changes to make

Create `backend-go/services/auth-service/internal/domain/pairing_session.go`:

```go
// Package domain holds auth-service's core business types — see
// existing domain/*.go files for the package-level convention.
package domain

import (
	"errors"
	"time"
)

var (
	ErrPairingTokenNotFound = errors.New("domain: pairing token not found")
	ErrPairingTokenExpired  = errors.New("domain: pairing token expired")     // BR-MB-01
	ErrPairingTokenConsumed = errors.New("domain: pairing token already used") // BR-MB-02
)

// PairingSession is the ephemeral server-side state of one in-progress QR
// pairing attempt (BL-MB-01). Expired/Consumed are the invariants
// InitiateDevicePairing/CompleteDevicePairing check before proceeding.
type PairingSession struct {
	ID                          string // == hash(pairing_token) — never the raw token, mirrors sessions.id
	TenantID, UserID            string
	DesktopPublicKey            []byte
	DesktopPrivateKeyCiphertext []byte // Vault Transit-encrypted — decrypted once, in CompleteDevicePairing
	VaultKeyRef                 string
	CreatedAt, ExpiresAt        time.Time
	ConsumedAt                  *time.Time // BR-MB-02: non-nil once consumed
}

func (s PairingSession) Expired(now time.Time) bool { return now.After(s.ExpiresAt) }
func (s PairingSession) Consumed() bool             { return s.ConsumedAt != nil }
```

Create `backend-go/services/auth-service/internal/domain/paired_device.go`:

```go
package domain

import "time"

var (
	ErrDeviceLimitReached = errors.New("domain: device pairing limit reached") // BR-MB-03
	ErrDeviceNotFound     = errors.New("domain: paired device not found")
)

// MaxPairedDevicesPerAccount is BR-MB-03's cap — enforced by the usecase
// counting active rows before insert (needs a repository query, not just
// this struct's own fields).
const MaxPairedDevicesPerAccount = 3

// DeviceStatus is a PairedDevice's lifecycle state.
type DeviceStatus string

const (
	DeviceActive  DeviceStatus = "active"
	DeviceRevoked DeviceStatus = "revoked"
)

// PairedDevice is a durably paired mobile device.
type PairedDevice struct {
	ID, TenantID, UserID   string
	DeviceLabel            string
	SharedSecretCiphertext []byte // Vault Transit-encrypted 32-byte NaCl box shared secret — never plaintext in this row
	VaultKeyRef            string
	Status                 DeviceStatus
	PairedAt, LastUsedAt   time.Time
	RevokedAt              *time.Time
}
```

Note: put the `errors` import at the top of `paired_device.go` alongside `time`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... && go vet ./services/auth-service/...
```
