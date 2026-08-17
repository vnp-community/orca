package usecase

import (
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter is a real, working per-tenant token-bucket rate limiter
// (api-gateway.md §9's "rate limiting... the first line of defense against
// abusive clients and downstream overload"). This is the per-replica
// in-memory form the design doc calls out as an acceptable choice (§5:
// "per-replica or backed by a shared fast store (Redis)... either way
// disposable counters, not business data"). Swapping in a Redis-backed
// RateLimitStore (ports.go) for cross-replica-consistent limiting is future
// work, not required for this to be a real limiter today.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

// NewRateLimiter returns a limiter allowing rps requests/sec sustained per
// tenant, bursting up to burst — each tenant ID gets its own independent
// token bucket, created lazily on first use.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// Allow reports whether tenantID may make a request right now, consuming
// one token from that tenant's bucket if so. Safe for concurrent use.
func (l *RateLimiter) Allow(tenantID string) bool {
	return l.limiterFor(tenantID).Allow()
}

func (l *RateLimiter) limiterFor(tenantID string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.limiters[tenantID]
	if !ok {
		lim = rate.NewLimiter(l.rps, l.burst)
		l.limiters[tenantID] = lim
	}
	return lim
}
