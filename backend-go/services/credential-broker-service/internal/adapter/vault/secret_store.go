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

	"github.com/stablyai/orca-go/common/secrets"
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

// RevokeSecret permanently deletes the material at path — both the current
// version and its full version history — so a subsequent KVRead can never
// again return usable data, including via KV v2's version-history read
// path. Delegates to secrets.Client.KVDestroyMetadata, Vault's native
// `DELETE <mount>/metadata/<path>` call. This replaces the prior
// overwrite-with-empty-payload workaround (which only added a new,
// recoverable version) now that common/secrets exposes the real delete —
// see that method's doc comment. Still does not touch the Transit key
// (Transit keys are shared across every credential in a category, so
// deleting one would revoke every credential in that category, not just
// this one); the fail-closed status check in
// internal/usecase/resolve_credential.go remains defense in depth alongside
// this Vault-side delete, not a substitute for it.
func (s *SecretStore) RevokeSecret(ctx context.Context, mount, path string) error {
	return s.client.KVDestroyMetadata(ctx, mount, path)
}

// Ping is a lightweight Vault-reachability signal for the health check
// wired up in cmd/server/main.go. Delegates to secrets.Client.Ping, a real
// Sys().Health() call, now that common/secrets exposes one — replacing the
// prior well-known-absent-KV-path heuristic (see that method's doc
// comment). A sealed or uninitialized Vault still answers Health() and so
// still counts as "reachable" here; only genuine unreachability/transport
// failure errors.
func (s *SecretStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx)
}
