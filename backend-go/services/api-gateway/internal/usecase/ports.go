// Package usecase holds api-gateway's cross-cutting request handling —
// auth validation, rate-limit decisioning, WS-bridge lifecycle — not
// per-domain business use cases, because this service has none. See
// specs/backend-go/services/api-gateway.md §6.
package usecase

import "context"

// JWKSClient fetches auth-service's public keys for real RS256 JWT
// signature verification. NOT IMPLEMENTED in this scaffold — see the
// PLACEHOLDER warning on AuthValidator in validate_identity.go. Defined
// here as the port a real internal/adapter/authclient implementation would
// satisfy, per api-gateway.md §6's package-layout sketch, once built.
type JWKSClient interface {
	// PublicKey returns the public key for the given key ID (kid), fetched
	// from auth-service's JWKS endpoint and cached with a short TTL
	// per api-gateway.md §5.
	PublicKey(ctx context.Context, kid string) (any, error)
}

// RateLimitStore is the storage port a shared, multi-replica rate limiter
// (Redis-backed, per api-gateway.md §5) would implement. RateLimiter in
// rate_limit.go is a real, working per-replica in-memory implementation
// that does not need this interface; RateLimitStore exists so a future
// shared store can be swapped in behind the same Allow(tenantID) decision
// shape without touching callers.
type RateLimitStore interface {
	Allow(ctx context.Context, tenantID string) (bool, error)
}
