package usecase

import (
	"context"
	"sync"
	"time"
)

// CoalescedEvent aggregates repeated index changed notifications into a single batch.
type CoalescedEvent struct {
	TenantID      string
	BindingID     string
	Tools         map[string]bool
	NewestCommit  string
	NewestIndexed string
	Kind          string
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
}

// IndexEventCoalescer groups events by (tenant, binding) to mitigate event storms (TASK-024-04).
type IndexEventCoalescer struct {
	debounce time.Duration
	maxWait  time.Duration
	handler  func(ctx context.Context, event CoalescedEvent)

	mu      sync.Mutex
	pending map[string]*CoalescedEvent
	timers  map[string]*time.Timer
}

// NewIndexEventCoalescer creates a new coalescer.
func NewIndexEventCoalescer(
	debounce time.Duration,
	maxWait time.Duration,
	handler func(ctx context.Context, event CoalescedEvent),
) *IndexEventCoalescer {
	if debounce <= 0 {
		debounce = 2 * time.Second
	}
	if maxWait <= 0 {
		maxWait = 10 * time.Second
	}
	return &IndexEventCoalescer{
		debounce: debounce,
		maxWait:  maxWait,
		handler:  handler,
		pending:  make(map[string]*CoalescedEvent),
		timers:   make(map[string]*time.Timer),
	}
}

func coalescerKey(tenant, binding string) string {
	return tenant + ":" + binding
}

// Ingest adds an event to the coalescer, resetting debounce or flushing if maxWait is reached.
func (c *IndexEventCoalescer) Ingest(ctx context.Context, tenant, binding, tool, commit, indexedAt, kind string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := coalescerKey(tenant, binding)
	entry, exists := c.pending[key]
	now := time.Now()

	if !exists {
		entry = &CoalescedEvent{
			TenantID:      tenant,
			BindingID:     binding,
			Tools:         make(map[string]bool),
			NewestCommit:  commit,
			NewestIndexed: indexedAt,
			Kind:          kind,
			FirstSeenAt:   now,
			LastSeenAt:    now,
		}
		if tool != "" {
			entry.Tools[tool] = true
		}
		c.pending[key] = entry
	} else {
		if tool != "" {
			entry.Tools[tool] = true
		}
		if commit != "" {
			entry.NewestCommit = commit
		}
		if indexedAt != "" {
			entry.NewestIndexed = indexedAt
		}
		entry.LastSeenAt = now
	}

	// If maxWait exceeded since first event, flush immediately
	if now.Sub(entry.FirstSeenAt) >= c.maxWait {
		c.flushLocked(ctx, key)
		return
	}

	// Reset debounce timer
	if timer, ok := c.timers[key]; ok {
		timer.Stop()
	}
	c.timers[key] = time.AfterFunc(c.debounce, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.flushLocked(ctx, key)
	})
}

func (c *IndexEventCoalescer) flushLocked(ctx context.Context, key string) {
	entry, ok := c.pending[key]
	if !ok {
		return
	}
	delete(c.pending, key)
	if timer, ok := c.timers[key]; ok {
		timer.Stop()
		delete(c.timers, key)
	}

	if c.handler != nil {
		go c.handler(ctx, *entry)
	}
}

// DevServerRateLimiter provides token bucket rate limiting (20 events/sec per dev server).
type DevServerRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64
	burst   int
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// NewDevServerRateLimiter creates a rate limiter with rate events/sec and burst capacity.
func NewDevServerRateLimiter(rate float64, burst int) *DevServerRateLimiter {
	if rate <= 0 {
		rate = 20.0
	}
	if burst <= 0 {
		burst = 20
	}
	return &DevServerRateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// Allow returns true if an event is permitted, false if rate limit exceeded.
func (r *DevServerRateLimiter) Allow(devServerID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.buckets[devServerID]
	now := time.Now()
	if !ok {
		b = &tokenBucket{
			tokens:     float64(r.burst) - 1.0,
			lastRefill: now,
		}
		r.buckets[devServerID] = b
		return true
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * r.rate
	if b.tokens > float64(r.burst) {
		b.tokens = float64(r.burst)
	}
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}
