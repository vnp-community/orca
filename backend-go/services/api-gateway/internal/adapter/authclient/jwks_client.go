package authclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// jwksCacheTTL bounds how stale a cached JWKS can be before a fresh
// GetJWKS call is made. auth-service.md §9's key-rotation overlap window is
// measured in minutes, so a few-minutes TTL here still catches a rotation
// promptly without hammering auth-service on every request needing a kid
// lookup (design doc: "cacheable aggressively (minutes)").
const jwksCacheTTL = 5 * time.Minute

// JWKSClient implements usecase.JWKSClient against auth-service's real
// GetJWKS RPC — unlike SessionValidator (a per-request server-side lookup),
// this is deliberately cached: GetJWKS is public/unauthenticated and its
// result changes only on key rotation, so paying a network round trip per
// verified JWT would be pure waste.
type JWKSClient struct {
	client authv1.AuthServiceClient

	mu        sync.Mutex
	cached    jose.JSONWebKeySet
	haveCache bool
	cachedAt  time.Time
}

// NewJWKSClient wraps an already-dialed connection to auth-service.
func NewJWKSClient(client authv1.AuthServiceClient) *JWKSClient {
	return &JWKSClient{client: client}
}

// PublicKey returns the public key for kid, fetched from auth-service's
// JWKS endpoint and cached with a short TTL (see jwksCacheTTL). Fails
// closed: an unknown kid, or a fetch error with nothing usable cached yet,
// returns an error rather than any key.
func (c *JWKSClient) PublicKey(ctx context.Context, kid string) (any, error) {
	set, err := c.freshSet(ctx)
	if err != nil {
		return nil, err
	}
	matches := set.Key(kid)
	if len(matches) == 0 {
		return nil, fmt.Errorf("authclient: no JWKS key found for kid %q", kid)
	}
	return matches[0].Key, nil
}

// freshSet returns the cached JWKS if it's within TTL, otherwise re-fetches
// from auth-service. A fetch failure while a cached set is still present is
// NOT treated as fatal — the stale-but-still-cached set is served instead,
// since a transient auth-service blip shouldn't take down JWT verification
// for every other service; the fetch error only surfaces when there is
// nothing cached yet.
func (c *JWKSClient) freshSet(ctx context.Context) (jose.JSONWebKeySet, error) {
	c.mu.Lock()
	if c.haveCache && time.Since(c.cachedAt) < jwksCacheTTL {
		defer c.mu.Unlock()
		return c.cached, nil
	}
	hadCache := c.haveCache
	stale := c.cached
	c.mu.Unlock()

	set, err := c.fetch(ctx)
	if err != nil {
		if hadCache {
			return stale, nil
		}
		return jose.JSONWebKeySet{}, err
	}

	c.mu.Lock()
	c.cached, c.haveCache, c.cachedAt = set, true, time.Now()
	c.mu.Unlock()
	return set, nil
}

func (c *JWKSClient) fetch(ctx context.Context) (jose.JSONWebKeySet, error) {
	resp, err := c.client.GetJWKS(ctx, &authv1.GetJWKSRequest{})
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("authclient: fetching JWKS: %w", err)
	}
	var set jose.JSONWebKeySet
	if err := json.Unmarshal([]byte(resp.GetJwksJson()), &set); err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("authclient: parsing JWKS: %w", err)
	}
	return set, nil
}
