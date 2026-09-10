// Package domain holds auth-service's core business types — see
// existing domain/*.go files for the package-level convention.
package domain

import (
	"errors"
	"time"
)

var (
	ErrPairingTokenNotFound = errors.New("domain: pairing token not found")
	ErrPairingTokenExpired  = errors.New("domain: pairing token expired")      // BR-MB-01
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
