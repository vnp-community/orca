// Package nacl implements usecase.E2ESealer via
// golang.org/x/crypto/nacl/secretbox — the same crypto_secretbox
// construction the mobile client's TweetNaCl.js implements client-side
// (BL-MB-01's security model). A paired device's shared secret (resolved
// via auth-service's ResolveDeviceSharedSecret, SOL-MB-01) is used here
// only to seal a push payload; it is never persisted or logged by this
// package.
package nacl

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"
)

// sharedSecretSize/nonceSize match TweetNaCl's crypto_secretbox key/nonce
// lengths — the mobile client (auth-service's SOL-MB-01 pairing handshake)
// and this sealer must agree on both exactly, or Open on the other side
// fails.
const (
	sharedSecretSize = 32
	nonceSize        = 24
)

// Sealer implements usecase.E2ESealer (defined in
// internal/usecase/deliver_push.go).
type Sealer struct{}

func New() *Sealer { return &Sealer{} }

// Seal encrypts plaintext with sharedSecret (must be exactly 32 bytes) —
// a fresh random 24-byte nonce is generated per call and returned
// alongside the ciphertext, since the mobile client needs the same nonce
// to call secretbox.Open.
func (Sealer) Seal(plaintext []byte, sharedSecret []byte) (ciphertext, nonce []byte, err error) {
	if len(sharedSecret) != sharedSecretSize {
		return nil, nil, fmt.Errorf("nacl: shared secret must be %d bytes, got %d", sharedSecretSize, len(sharedSecret))
	}
	var key [sharedSecretSize]byte
	copy(key[:], sharedSecret)

	var n [nonceSize]byte
	if _, err := rand.Read(n[:]); err != nil {
		return nil, nil, fmt.Errorf("nacl: generating nonce: %w", err)
	}

	sealed := secretbox.Seal(nil, plaintext, &n, &key)
	return sealed, n[:], nil
}
