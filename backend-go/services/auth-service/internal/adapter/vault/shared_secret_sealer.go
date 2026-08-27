package vault

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/secrets"
)

// sharedSecretKeyType is the Transit key type Ensure creates
// sharedSecretKeyName as, if it doesn't already exist — AES-GCM is the
// right primitive for symmetric-payload encryption (as opposed to
// TokenSigner's RSA signing key).
const (
	sharedSecretKeyName = "device-shared-secret"
	sharedSecretKeyType = "aes256-gcm96"
)

// SharedSecretSealer implements usecase.SharedSecretSealer over a
// *secrets.Client's Transit encrypt/decrypt data operations — never against
// Vault directly, extending the "no adapter/vault/... only path is the
// mediated Transit call" rule this package already applies to JWT signing
// (token_signer.go) to a second Transit operation.
type SharedSecretSealer struct {
	client *secrets.Client
}

func NewSharedSecretSealer(client *secrets.Client) *SharedSecretSealer {
	return &SharedSecretSealer{client: client}
}

// Ensure creates the device-shared-secret Transit key if missing — call
// once at startup alongside TokenSigner.Ensure.
func (s *SharedSecretSealer) Ensure(ctx context.Context) error {
	if err := s.client.TransitEnsureKey(ctx, sharedSecretKeyName, sharedSecretKeyType); err != nil {
		return fmt.Errorf("vault: ensuring %s transit key: %w", sharedSecretKeyName, err)
	}
	return nil
}

// Encrypt seals plaintext through Vault Transit — the returned keyRef
// (always sharedSecretKeyName today) is stored alongside the ciphertext so
// Decrypt doesn't have to assume a fixed key name, matching
// domain.PairingSession/PairedDevice's own VaultKeyRef field.
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
