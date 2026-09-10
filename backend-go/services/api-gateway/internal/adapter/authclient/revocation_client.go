package authclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// revocationCacheTTL bounds how stale a cached revocation decision can be
// — mirrors jwksCacheTTL's reasoning (jwks_client.go): short enough that a
// just-revoked CLI token stops working promptly, long enough that a busy
// bearer-JWT caller doesn't cost auth-service a round trip on every
// request. CR-CLI-002/TASK-BE-CLI-005 explicitly calls out this cache as
// "bắt buộc" (mandatory) — do not remove it.
const revocationCacheTTL = 5 * time.Minute

// revocationCacheEntry is one cached (jti -> revoked) decision.
type revocationCacheEntry struct {
	revoked  bool
	cachedAt time.Time
}

// RevocationClient implements usecase.RevocationChecker against
// auth-service's real IsServiceTokenRevoked RPC, per-jti cached with a
// short TTL (revocationCacheTTL) — see that RPC's proto doc comment for
// why it has no caller-identity gate (a jti reveals nothing about its
// owning user).
//
// Unlike JWKSClient's single shared cache entry, this caches one entry per
// jti — bounded in practice by revocationCacheTTL: a jti's entry is only
// ever consulted while its own JWT is still unexpired (a few minutes to
// hours per TokenSigner's TTL), so this map cannot grow without bound the
// way an unbounded cache of every jti ever seen would.
type RevocationClient struct {
	client authv1.AuthServiceClient

	mu    sync.Mutex
	cache map[string]revocationCacheEntry
}

// NewRevocationClient wraps an already-dialed connection to auth-service.
func NewRevocationClient(client authv1.AuthServiceClient) *RevocationClient {
	return &RevocationClient{client: client, cache: make(map[string]revocationCacheEntry)}
}

// IsRevoked returns whether jti has been revoked, serving a cached
// decision within revocationCacheTTL instead of calling auth-service on
// every request. Unlike JWKSClient.freshSet, a fetch failure has no stale
// fallback here — see AuthValidator.Validate's fail-closed treatment of
// this method's error return.
func (c *RevocationClient) IsRevoked(ctx context.Context, jti string) (bool, error) {
	c.mu.Lock()
	if entry, ok := c.cache[jti]; ok && time.Since(entry.cachedAt) < revocationCacheTTL {
		c.mu.Unlock()
		return entry.revoked, nil
	}
	c.mu.Unlock()

	resp, err := c.client.IsServiceTokenRevoked(ctx, &authv1.IsServiceTokenRevokedRequest{Jti: jti})
	if err != nil {
		return false, fmt.Errorf("authclient: checking token revocation: %w", err)
	}

	c.mu.Lock()
	c.cache[jti] = revocationCacheEntry{revoked: resp.GetRevoked(), cachedAt: time.Now()}
	c.mu.Unlock()
	return resp.GetRevoked(), nil
}
