package tools

import (
	"sync"
	"time"
)

// defaultRequestCreatePerHour caps Requests one MCP client may create for a user
// per hour; request_create and request_spawnChild share the budget.
const defaultRequestCreatePerHour = 20

// requestCreateTools are the tools that spend the creation budget.
func spendsRequestCreateBudget(toolName string) bool {
	return toolName == "request_create" || toolName == "request_spawnChild"
}

// requestCreateLimiter is an in-process token bucket per (tenant, user, client).
// It limits per gateway replica, so N replicas allow N times the rate; the hard
// cap is request-service's pending limit. Memory is bounded like scmLimiter.
type requestCreateLimiter struct {
	mu      sync.Mutex
	perHour int
	buckets map[string]*rateBucket
	now     func() time.Time
}

func newRequestCreateLimiter(perHour int, now func() time.Time) *requestCreateLimiter {
	return &requestCreateLimiter{perHour: perHour, buckets: map[string]*rateBucket{}, now: now}
}

// allow spends one token; on refusal it returns how long until one is free.
func (l *requestCreateLimiter) allow(tenant, user, client string) (bool, time.Duration) {
	if l == nil || l.perHour <= 0 {
		return true, 0
	}
	key := tenant + "\x00" + user + "\x00" + client
	rate := float64(l.perHour) / 3600 // tokens per second
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= maxRateBuckets {
			l.evict(now)
		}
		b = &rateBucket{tokens: float64(l.perHour), last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * rate
	if b.tokens > float64(l.perHour) {
		b.tokens = float64(l.perHour)
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / rate * float64(time.Second))
}

// evict drops buckets that have fully refilled (forgetting them loses nothing),
// then arbitrary ones so the map never exceeds maxRateBuckets.
func (l *requestCreateLimiter) evict(now time.Time) {
	full := time.Hour
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
	for k := range l.buckets {
		if len(l.buckets) < maxRateBuckets {
			return
		}
		delete(l.buckets, k)
	}
}
