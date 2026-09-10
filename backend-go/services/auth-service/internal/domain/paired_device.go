package domain

import (
	"errors"
	"time"
)

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
