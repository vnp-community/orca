// Package vaultsigner implements usecase.VaultSigner against Vault's
// Transit engine via common/secrets — notification-service.md §9's
// headline property: the VAPID private key is generated inside Vault
// Transit and never leaves it; signing a push payload's VAPID JWT is a
// "Vault: sign" call, not "read secret, then sign locally."
//
// common/secrets exposes TransitEncrypt/TransitDecrypt today, not a
// dedicated Transit "sign" operation — this adapter uses TransitEncrypt as
// the available Transit-engine equivalent for producing signed-payload
// material without the plaintext key ever leaving Vault. Swap this for a
// real Transit `sign` call (Vault's asymmetric-key signing endpoint) once
// common/secrets grows one; see this service's README "Known gaps".
package vaultsigner

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/secrets"
)

// Signer implements usecase.VaultSigner. A nil *secrets.Client is
// accepted (e.g. when Vault isn't configured for local dev) so
// construction never fails at startup — SignVapidPayload returns a clear,
// wrapped error at call time instead, per this service's README note on
// graceful degradation when Vault is unreachable.
type Signer struct {
	client *secrets.Client
}

// New wraps client. client may be nil.
func New(client *secrets.Client) *Signer {
	return &Signer{client: client}
}

// SignVapidPayload signs payload under the tenant's VAPID Transit key.
// The key name convention ("vapid-signing-<tenant_id>") matches
// vapid_key_metadata.vault_key_ref (notification-service.md §5) — this
// adapter recomputes rather than reads that column: reading the column
// isn't itself a capability (§9), so recomputing the well-known name here
// avoids a needless repository round trip.
func (s *Signer) SignVapidPayload(ctx context.Context, tenantID string, payload []byte) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("vaultsigner: no vault client configured (VAULT_ADDR unset, or vault unreachable at startup) — VAPID payload signing is unavailable until Vault is reachable")
	}
	keyName := "vapid-signing-" + tenantID
	signed, err := s.client.TransitEncrypt(ctx, keyName, payload)
	if err != nil {
		return "", fmt.Errorf("vaultsigner: transit sign via key %s: %w", keyName, err)
	}
	return signed, nil
}
