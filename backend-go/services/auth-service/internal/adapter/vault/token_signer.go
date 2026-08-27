// Package vault implements usecase.TokenSigner as a thin wrapper over
// common/jwtauth.TransitSigner, itself backed by a *secrets.Client — the
// Vault Transit RSA key that signs every auth-service-issued JWT. The
// private key never enters this process's memory; only Vault-computed
// signatures and public-key material do (see jwtauth.TransitSigner's doc
// comment).
package vault

import (
	"context"
	"fmt"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/common/secrets"
)

// keyName is the single, global (not per-tenant) Vault Transit key
// auth-service signs every JWT with — see this service's README "Known
// gaps" for why this is intentionally global rather than per-tenant.
const keyName = "jwt-signing"

// keyType is the Transit key type Ensure creates keyName as if it doesn't
// already exist.
const keyType = "rsa-2048"

// TokenSigner implements usecase.TokenSigner over a *secrets.Client.
type TokenSigner struct {
	client *secrets.Client
	signer *jwtauth.TransitSigner
}

// New wraps client. Constructed once, in cmd/server/main.go, from a real
// secrets.NewClient() — see that file's composition-root comment. Callers
// must call Ensure before relying on Sign/PublicJWKS (see Ensure's doc
// comment).
func New(client *secrets.Client) *TokenSigner {
	return &TokenSigner{
		client: client,
		signer: jwtauth.NewTransitSigner(keyName, client.TransitSign, client.TransitPublicKeyVersions),
	}
}

// Ensure creates the jwt-signing Transit key if it doesn't already exist.
// Called once, at startup, from cmd/server/main.go — failing loudly (the
// server must not start) rather than deferring the failure to the first
// IssueServiceToken/GetJWKS call, per this task's "fail startup loudly"
// requirement.
func (s *TokenSigner) Ensure(ctx context.Context) error {
	if err := s.client.TransitEnsureKey(ctx, keyName, keyType); err != nil {
		return fmt.Errorf("vault: ensuring %s transit key: %w", keyName, err)
	}
	return nil
}

// Sign mints a compact-serialized RS256 JWT for claims via Vault Transit.
func (s *TokenSigner) Sign(ctx context.Context, claims jwtauth.Claims) (string, error) {
	return jwtauth.Sign(ctx, s.signer, claims)
}

// PublicJWKS returns the current+previous signing key version as an RFC
// 7517 JWK Set — see jwtauth.TransitSigner.PublicJWKS's doc comment for the
// rotation-overlap rationale.
func (s *TokenSigner) PublicJWKS(ctx context.Context) (jose.JSONWebKeySet, error) {
	return s.signer.PublicJWKS(ctx)
}

// Ping is a lightweight Vault-reachability signal for the health check
// wired up in cmd/server/main.go — mirrors credential-broker-service's
// SecretStore.Ping in spirit, but simpler: unlike that service's
// intentionally-absent probe path, jwt-signing is a real key Ensure
// creates at startup, so reading its public key is itself a meaningful
// reachability check with no "not found means reachable" heuristic needed.
func (s *TokenSigner) Ping(ctx context.Context) error {
	_, _, err := s.client.TransitPublicKey(ctx, keyName)
	if err != nil {
		return fmt.Errorf("vault unreachable or misconfigured: %w", err)
	}
	return nil
}
