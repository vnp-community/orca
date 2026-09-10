// mobile_envelope.go holds the E2E NaCl-secretbox seal/open helpers shared
// by every mobile.* channel (channels_mobile_dispatch.go's decrypt side,
// channels_mobile_status.go's encrypt side) — factored into one file per
// both tasks' doc comments, rather than duplicating the base64-decode +
// secretbox call in two places. Mirrors
// notification-service/internal/adapter/nacl.Sealer's construction (same
// 32-byte shared secret / 24-byte nonce sizes, since both sides seal for
// the same TweetNaCl.js mobile client, BL-MB-01's security model).
package wscompat

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	mobileSharedSecretSize = 32
	mobileNonceSize        = 24
)

// mobileEnvelope is the wire shape for every mobile.* channel's encrypted
// payload — base64 ciphertext + nonce, BR-MB-13's "always sealed, never raw
// JSON" requirement.
type mobileEnvelope struct {
	Ciphertext string `json:"ciphertext"`
	Nonce      string `json:"nonce"`
}

// unsealMobilePayload base64-decodes ciphertext/nonce and NaCl-secretbox-opens
// them against sharedSecret, returning the decrypted plaintext prompt string
// — used by channels_mobile_dispatch.go's mobile.dispatch handler.
func unsealMobilePayload(ciphertextB64, nonceB64 string, sharedSecret []byte) (string, error) {
	if len(sharedSecret) != mobileSharedSecretSize {
		return "", fmt.Errorf("wscompat: shared secret must be %d bytes, got %d", mobileSharedSecretSize, len(sharedSecret))
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("wscompat: decoding ciphertext: %w", err)
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", fmt.Errorf("wscompat: decoding nonce: %w", err)
	}
	if len(nonceBytes) != mobileNonceSize {
		return "", fmt.Errorf("wscompat: nonce must be %d bytes, got %d", mobileNonceSize, len(nonceBytes))
	}

	var key [mobileSharedSecretSize]byte
	copy(key[:], sharedSecret)
	var nonce [mobileNonceSize]byte
	copy(nonce[:], nonceBytes)

	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &key)
	if !ok {
		return "", fmt.Errorf("wscompat: secretbox open failed (bad key/nonce or tampered ciphertext)")
	}
	return string(plaintext), nil
}

// sealMobileEnvelope marshals resp to JSON and NaCl-secretbox-seals it with
// sharedSecret, returning the {ciphertext, nonce} envelope (base64) BR-MB-13
// requires — used by channels_mobile_status.go's mobile.status /
// mobile.statusSubscribe handlers. resp is `any` (rather than proto.Message)
// so this helper works for any JSON-marshalable response shape, not just
// generated proto messages.
func sealMobileEnvelope(resp any, sharedSecret []byte) (any, error) {
	if len(sharedSecret) != mobileSharedSecretSize {
		return nil, fmt.Errorf("wscompat: shared secret must be %d bytes, got %d", mobileSharedSecretSize, len(sharedSecret))
	}
	plaintext, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("wscompat: marshaling mobile envelope payload: %w", err)
	}

	var key [mobileSharedSecretSize]byte
	copy(key[:], sharedSecret)
	var nonce [mobileNonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("wscompat: generating nonce: %w", err)
	}

	sealed := secretbox.Seal(nil, plaintext, &nonce, &key)
	return mobileEnvelope{
		Ciphertext: base64.StdEncoding.EncodeToString(sealed),
		Nonce:      base64.StdEncoding.EncodeToString(nonce[:]),
	}, nil
}
