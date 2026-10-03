package tools

import (
	"sync"
	"time"
)

const maxRateBuckets = 10000

// rateGroup maps a tool namespace to the provider quota it spends; "" = unlimited.
func rateGroup(namespace string) string {
	switch namespace {
	case "github", "gitlab", "linear", "hostedReview":
		return namespace
	}
	return ""
}

// scmLimiter is an in-process token bucket per (tenant, provider group).
// Memory is bounded: idle (refilled) buckets are dropped first.
type scmLimiter struct {
	mu      sync.Mutex
	perMin  int
	buckets map[string]*rateBucket
	now     func() time.Time
}

type rateBucket struct {
	tokens float64
	last   time.Time
}

func newSCMLimiter(perMin int, now func() time.Time) *scmLimiter {
	return &scmLimiter{perMin: perMin, buckets: map[string]*rateBucket{}, now: now}
}

// allow spends one token; on refusal it returns how long until one is free.
func (l *scmLimiter) allow(tenant, group string) (bool, time.Duration) {
	if l == nil || l.perMin <= 0 || group == "" {
		return true, 0
	}
	key := tenant + "\x00" + group
	rate := float64(l.perMin) / 60 // tokens per second
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= maxRateBuckets {
			l.evict(now)
		}
		b = &rateBucket{tokens: float64(l.perMin), last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * rate
	if b.tokens > float64(l.perMin) {
		b.tokens = float64(l.perMin)
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / rate * float64(time.Second))
}

func (l *scmLimiter) evict(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) >= time.Minute { // fully refilled: forgetting it loses nothing
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
