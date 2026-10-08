package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	maxRateBuckets  = 10000
	rateBucketIdle  = 10 * time.Minute
	rateSweepPeriod = time.Minute
)

type rateBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a token bucket per (tenant, class[, subject]). It lives in memory, so N replicas allow N
// times the rate; the DB-backed concurrency caps are the exact limits.
type RateLimiter struct {
	limits map[string]domain.Limit
	now    func() time.Time

	mu        sync.Mutex
	buckets   map[string]*rateBucket
	lastSweep time.Time
}

func NewRateLimiter(limits map[string]domain.Limit, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}
	return &RateLimiter{limits: limits, now: now, buckets: map[string]*rateBucket{}}
}

// Allow takes one token; when none is left it reports how long until one is. subject narrows the bucket
// (an MCP client gets its own) and may be empty. A class without a limit is not limited.
func (l *RateLimiter) Allow(tenantID, class, subject string) (bool, time.Duration) {
	lim, ok := l.limits[class]
	if !ok || lim.PerSecond <= 0 {
		return true, 0
	}
	now := l.now()
	key := tenantID + "\x00" + class + "\x00" + subject

	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(now)
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= maxRateBuckets {
			l.evictOldestLocked()
		}
		b = &rateBucket{tokens: float64(lim.Burst), last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * lim.PerSecond
	if max := float64(lim.Burst); b.tokens > max {
		b.tokens = max
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / lim.PerSecond * float64(time.Second))
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	return false, wait
}

func (l *RateLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < rateSweepPeriod {
		return
	}
	l.lastSweep = now
	for k, b := range l.buckets {
		if now.Sub(b.last) > rateBucketIdle {
			delete(l.buckets, k)
		}
	}
}

func (l *RateLimiter) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for k, b := range l.buckets {
		if oldestKey == "" || b.last.Before(oldest) {
			oldestKey, oldest = k, b.last
		}
	}
	delete(l.buckets, oldestKey)
}

// Size is the number of live buckets (tests and metrics).
func (l *RateLimiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// ConcurrencyGate checks the DB-backed caps before work that is expensive or unbounded.
type ConcurrencyGate struct {
	caps    domain.ConcurrencyCaps
	counter ConcurrencyCounter
}

func NewConcurrencyGate(caps domain.ConcurrencyCaps, counter ConcurrencyCounter) *ConcurrencyGate {
	return &ConcurrencyGate{caps: caps, counter: counter}
}

// CheckCreate refuses a new Request when the tenant already has too many open ones.
func (g *ConcurrencyGate) CheckCreate(ctx context.Context) error {
	n, err := g.counter.CountOpenRequests(ctx)
	if err != nil {
		return err
	}
	if g.caps.OpenRequestsPerTenant > 0 && n >= g.caps.OpenRequestsPerTenant {
		return &domain.RateLimitedError{Class: domain.RateWrite, Detail: "concurrency", RetryAfter: 30 * time.Second}
	}
	return nil
}

// CheckRun refuses starting another AI run when the tenant or the project is at its running cap.
func (g *ConcurrencyGate) CheckRun(ctx context.Context, projectID string) error {
	n, err := g.counter.CountRunning(ctx, "")
	if err != nil {
		return err
	}
	if g.caps.RunningPerTenant > 0 && n >= g.caps.RunningPerTenant {
		return &domain.RateLimitedError{Class: domain.RateAI, Detail: "concurrency", RetryAfter: 15 * time.Second}
	}
	if projectID == "" || g.caps.RunningPerProject <= 0 {
		return nil
	}
	n, err = g.counter.CountRunning(ctx, projectID)
	if err != nil {
		return err
	}
	if n >= g.caps.RunningPerProject {
		return &domain.RateLimitedError{Class: domain.RateAI, Detail: "concurrency", RetryAfter: 15 * time.Second}
	}
	return nil
}
