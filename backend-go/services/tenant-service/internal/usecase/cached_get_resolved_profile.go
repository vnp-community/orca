package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// DefaultProfileCacheTTL is the staleness bound tenant-service.md §6 keeps
// for behavioral continuity with the TS system's process-local 60s TTL
// cache. Each replica has its own cache, so an update on one replica
// doesn't invalidate another's cache within this window — an accepted
// staleness bound, not a bug (tenant-service.md §6/§8; see README "Known
// gaps").
const DefaultProfileCacheTTL = 60 * time.Second

// CachedGetResolvedProfile decorates GetResolvedProfile with a
// cache-first read path — a usecase-layer decorator wrapping the base
// usecase, not a change to GetResolvedProfile itself or to
// internal/adapter/postgres, per tenant-service.md §6's explicit design
// choice ("in-process LRU-with-TTL, not a shared read-through cache").
type CachedGetResolvedProfile struct {
	base  *GetResolvedProfile
	cache ProfileCache
	ttl   time.Duration
}

// NewCachedGetResolvedProfile wraps base with cache. A non-positive ttl
// defaults to DefaultProfileCacheTTL.
func NewCachedGetResolvedProfile(base *GetResolvedProfile, cache ProfileCache, ttl time.Duration) *CachedGetResolvedProfile {
	if ttl <= 0 {
		ttl = DefaultProfileCacheTTL
	}
	return &CachedGetResolvedProfile{base: base, cache: cache, ttl: ttl}
}

func (d *CachedGetResolvedProfile) Execute(ctx context.Context, userID string) (domain.ResolvedProfile, error) {
	if cached, ok := d.cache.Get(ctx, userID); ok {
		return cached, nil
	}

	resolved, err := d.base.Execute(ctx, userID)
	if err != nil {
		return domain.ResolvedProfile{}, err
	}

	d.cache.Set(ctx, userID, resolved, d.ttl)
	return resolved, nil
}
