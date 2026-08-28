# TASK-MB-01-04: Add `DeviceKeyExchanger` (NaCl box) and `SharedSecretSealer` (Vault Transit) adapters

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `auth-service`
**File:** `backend-go/services/auth-service/internal/adapter/nacl/box.go`, `backend-go/services/auth-service/internal/adapter/vault/shared_secret_sealer.go`
**Depends on:** TASK-MB-01-02
**Status:** `[ ]` TODO

---

## Context

`InitiateDevicePairing`/`CompleteDevicePairing` need two new ports beyond
auth-service's existing `TokenSigner`/`PasswordHasher`: a NaCl X25519
keypair/shared-secret generator, and a way to encrypt/decrypt that shared
secret at rest without it ever touching this process's disk in plaintext.
`common/secrets.Client` already exposes `TransitEncrypt`/`TransitDecrypt`
(used today by no service yet for a non-signing purpose) — `SharedSecretSealer`
wraps those directly, the same mediation pattern `internal/adapter/vault/token_signer.go`
already uses for `TransitSign`.

## Changes to make

`backend-go/services/auth-service/internal/adapter/nacl/box.go`:

```go
// Package nacl implements usecase.DeviceKeyExchanger over
// golang.org/x/crypto/nacl/box — the same NaCl crypto_box construction
// TweetNaCl.js implements client-side (BL-MB-01's security model), so no
// protocol redesign is needed on either side.
package nacl

import "golang.org/x/crypto/nacl/box"

type KeyExchanger struct{}

func New() *KeyExchanger { return &KeyExchanger{} }

// GenerateEphemeralKeypair returns a fresh X25519 keypair for one pairing
// session — never reused across sessions.
func (KeyExchanger) GenerateEphemeralKeypair() (pub, priv []byte, err error) {
	pubKey, privKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return pubKey[:], privKey[:], nil
}

// SharedSecret computes the box.Precompute shared secret from this side's
// private key and the peer's public key — the desktop-side half of
// BL-MB-01's handshake diagram; the mobile side computes the byte-identical
// value via TweetNaCl's box.before.
func (KeyExchanger) SharedSecret(priv, peerPub []byte) ([]byte, error) {
	if len(priv) != 32 || len(peerPub) != 32 {
		return nil, fmt.Errorf("nacl: key must be 32 bytes")
	}
	var privArr, pubArr [32]byte
	copy(privArr[:], priv)
	copy(pubArr[:], peerPub)
	var shared [32]byte
	box.Precompute(&shared, &pubArr, &privArr)
	return shared[:], nil
}
```

Add `"crypto/rand"` and `"fmt"` to the import block.

`backend-go/services/auth-service/internal/adapter/vault/shared_secret_sealer.go`:

```go
// SharedSecretSealer implements usecase.SharedSecretSealer over
// common/secrets.Client's Transit encrypt/decrypt data operations — never
// against Vault directly, extending the "no adapter/vault/... only path is
// the mediated Transit call" rule this package already applies to JWT
// signing (token_signer.go) to a second Transit operation.
package vault

import "context"

const sharedSecretKeyName = "device-shared-secret"

type SharedSecretSealer struct {
	client *secrets.Client
}

func NewSharedSecretSealer(client *secrets.Client) *SharedSecretSealer {
	return &SharedSecretSealer{client: client}
}

// Ensure creates the device-shared-secret Transit key if missing — call
// once at startup alongside TokenSigner.Ensure.
func (s *SharedSecretSealer) Ensure(ctx context.Context) error {
	return s.client.TransitEnsureKey(ctx, sharedSecretKeyName, "aes256-gcm96")
}

func (s *SharedSecretSealer) Encrypt(ctx context.Context, plaintext []byte) (ciphertext []byte, keyRef string, err error) {
	ct, err := s.client.TransitEncrypt(ctx, sharedSecretKeyName, plaintext)
	if err != nil {
		return nil, "", fmt.Errorf("vault: sealing shared secret: %w", err)
	}
	return []byte(ct), sharedSecretKeyName, nil
}

func (s *SharedSecretSealer) Decrypt(ctx context.Context, ciphertext []byte, keyRef string) ([]byte, error) {
	pt, err := s.client.TransitDecrypt(ctx, keyRef, string(ciphertext))
	if err != nil {
		return nil, fmt.Errorf("vault: unsealing shared secret: %w", err)
	}
	return pt, nil
}
```

Add the two ports to `usecase/ports.go`, joining the existing `PasswordHasher`/`TokenSigner` list:

```go
// DeviceKeyExchanger generates NaCl X25519 keypairs and computes shared
// secrets for BL-MB-01's pairing handshake.
type DeviceKeyExchanger interface {
	GenerateEphemeralKeypair() (pub, priv []byte, err error)
	SharedSecret(priv, peerPub []byte) ([]byte, error)
}

// SharedSecretSealer mediates a paired device's shared secret through
// Vault Transit — never a plaintext value in this service's own Postgres
// row, mirroring notification-service's/infra-fleet-service's Vault-mediated
// secret pattern extended here to a per-device-pairing secret class.
type SharedSecretSealer interface {
	Encrypt(ctx context.Context, plaintext []byte) (ciphertext []byte, keyRef string, err error)
	Decrypt(ctx context.Context, ciphertext []byte, keyRef string) ([]byte, error)
}
```

Wire `SharedSecretSealer.Ensure` into `cmd/server/main.go`'s startup sequence, next to the existing `TokenSigner.Ensure` call.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... && go vet ./services/auth-service/...
```

Add a round-trip unit test in `internal/adapter/nacl/box_test.go` asserting
two independently-generated keypairs (desktop, mobile) produce a
byte-identical `SharedSecret` from each side (`desktop.SharedSecret(desktopPriv, mobilePub) == mobile.SharedSecret(mobilePriv, desktopPub)`).
