// Package vault implements credential-broker-service's usecase.SecretStore
// port as a thin pass-through over common/secrets.Client — this is the ONE
// service in the whole backend-go scaffold where common/secrets' Transit
// and KV v2 methods are wired for real, non-stub use. Every other service's
// Vault identity is scoped only to its own dynamic DB credential lease (the
// bootstrap exception, see common/secrets' doc comment and
// credential-broker-service.md §2/§9) — this package is what a Vault policy
// granting tenant-secret-path access is actually built for.
package vault

import (
	"context"
	"errors"
	"strings"

	"github.com/stablyai/orca-go/common/secrets"
)

// healthCheckMount/healthCheckPath are the well-known, expected-to-be-absent
// KV v2 location Ping reads to signal reachability — see Ping's doc comment.
const (
	healthCheckMount = "credential-secrets"
	healthCheckPath  = "_health_check"
)

// SecretStore wraps a *secrets.Client. Every method below delegates
// directly to the matching common/secrets method — no reinterpretation, no
// caching, no plaintext retained past the call, preserving
// credential-broker-service.md §9's "plaintext-in-memory-only-for-request-
// duration" guarantee (this wrapper adds no state of its own that could
// violate it).
type SecretStore struct {
	client *secrets.Client
}

// New wraps client. Constructed once, in cmd/server/main.go, from a real
// secrets.NewClient() — see that file's composition-root comment.
func New(client *secrets.Client) *SecretStore {
	return &SecretStore{client: client}
}

// TransitEncrypt delegates directly to secrets.Client.TransitEncrypt.
func (s *SecretStore) TransitEncrypt(ctx context.Context, keyName string, plaintext []byte) (string, error) {
	return s.client.TransitEncrypt(ctx, keyName, plaintext)
}

// TransitDecrypt delegates directly to secrets.Client.TransitDecrypt.
func (s *SecretStore) TransitDecrypt(ctx context.Context, keyName, ciphertext string) ([]byte, error) {
	return s.client.TransitDecrypt(ctx, keyName, ciphertext)
}

// KVWrite delegates directly to secrets.Client.KVWrite.
func (s *SecretStore) KVWrite(ctx context.Context, mount, path string, data map[string]any) error {
	return s.client.KVWrite(ctx, mount, path, data)
}

// KVRead delegates directly to secrets.Client.KVRead.
func (s *SecretStore) KVRead(ctx context.Context, mount, path string) (map[string]any, error) {
	return s.client.KVRead(ctx, mount, path)
}

// RevokeSecret invalidates the material at path so a subsequent KVRead no
// longer returns usable data.
//
// KNOWN LIMITATION (documented here and in this service's README, not
// hidden): common/secrets.Client has no native KV-delete-version or
// Transit-key-delete method today, and this service must not modify
// common/ to add one. This method therefore calls the REAL
// secrets.Client.KVWrite with an empty payload — a genuine Vault call, not
// a stub — which overwrites the version KVRead would otherwise find. This
// is NOT equivalent to Vault's native "destroy version" API (which
// permanently scrubs the storage backend; an overwrite still leaves the
// prior version recoverable via KV v2's version history to anyone with
// broader-than-"current version" read access) and does not touch the
// Transit key at all (Transit keys are shared across every credential in a
// category, so deleting one would revoke every credential in that
// category, not just this one). The primary revocation enforcement in this
// service is therefore the fail-closed status check in
// internal/usecase/resolve_credential.go — this call is defense in depth on
// the Vault side, not the sole mechanism.
func (s *SecretStore) RevokeSecret(ctx context.Context, mount, path string) error {
	return s.client.KVWrite(ctx, mount, path, map[string]any{})
}

// Ping is a lightweight Vault-reachability signal for the health check
// wired up in cmd/server/main.go.
//
// common/secrets.Client has no dedicated health/ping method (adding one is
// out of scope for this service, which must not modify common/). Instead,
// this reads a well-known KV v2 path this service's own Vault policy would
// grant it access to, but which is never expected to actually exist: a
// "not found" error means Vault answered the request at all (reachable,
// authenticated, KV mount exists) — any other error (connection refused,
// sealed, permission denied, TLS failure) means Vault is NOT usable right
// now. This is a documented heuristic, not the most precise option; a
// production hardening pass should switch to a real
// client.Sys().Health()-style call once common/secrets exposes one. See
// this service's README "Known gaps".
func (s *SecretStore) Ping(ctx context.Context) error {
	_, err := s.client.KVRead(ctx, healthCheckMount, healthCheckPath)
	if err == nil {
		return nil // path unexpectedly exists — Vault is definitely reachable
	}
	if strings.Contains(err.Error(), "not found") {
		return nil // Vault reachable and answered; the probe path just doesn't exist, as expected
	}
	return errors.New("vault unreachable or misconfigured: " + err.Error())
}
