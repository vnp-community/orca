// Package cache implements usecase.ProfileCache as an in-process LRU cache
// with per-entry TTL — the design choice documented in tenant-service.md §6:
// the cached object is small, per-user, and its invalidation set is always
// known exactly at write time (SetUserDepartment/AddTeamMember each know
// exactly which user_id they affect), so a shared read-through cache
// (Redis) isn't justified. Each replica has its own cache; cross-replica
// staleness is bounded by TTL, not eliminated — see this service's README
// "Known gaps".
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// DefaultCapacity is a reasonable per-replica ceiling on cached profiles —
// the cached object is small (tenant-service.md §6), so this trades a
// modest amount of memory for avoiding unbounded growth under a
// high-cardinality user base.
const DefaultCapacity = 10_000

type entry struct {
	userID    string
	profile   domain.ResolvedProfile
	expiresAt time.Time
}

// LRUTTLCache is a fixed-capacity, mutex-guarded LRU cache keyed by userID,
// with lazy TTL expiry checked on Get. Implements usecase.ProfileCache.
type LRUTTLCache struct {
	mu       sync.Mutex
	capacity int
	order    *list.List // front = most recently used
	items    map[string]*list.Element
}

// NewLRUTTLCache constructs a cache holding at most capacity entries. A
// non-positive capacity defaults to DefaultCapacity.
func NewLRUTTLCache(capacity int) *LRUTTLCache {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &LRUTTLCache{
		capacity: capacity,
		order:    list.New(),
		items:    make(map[string]*list.Element),
	}
}

// Get returns userID's cached profile if present and not expired. An
// expired entry is evicted lazily on this call, not by a background sweep.
func (c *LRUTTLCache) Get(ctx context.Context, userID string) (domain.ResolvedProfile, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[userID]
	if !ok {
		return domain.ResolvedProfile{}, false
	}
	e := el.Value.(*entry)
	if time.Now().After(e.expiresAt) {
		c.removeLocked(el)
		return domain.ResolvedProfile{}, false
	}
	c.order.MoveToFront(el)
	return e.profile, true
}

// Set stores profile for userID with the given ttl, evicting the
// least-recently-used entry if the cache is over capacity.
func (c *LRUTTLCache) Set(ctx context.Context, userID string, profile domain.ResolvedProfile, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[userID]; ok {
		e := el.Value.(*entry)
		e.profile = profile
		e.expiresAt = time.Now().Add(ttl)
		c.order.MoveToFront(el)
		return
	}

	el := c.order.PushFront(&entry{userID: userID, profile: profile, expiresAt: time.Now().Add(ttl)})
	c.items[userID] = el

	if c.order.Len() > c.capacity {
		if oldest := c.order.Back(); oldest != nil {
			c.removeLocked(oldest)
		}
	}
}

// Invalidate evicts userID's cached profile immediately, if present — the
// synchronous half of tenant-service.md §8's invalidation-correctness
// requirement: mutating usecases call this on the serving replica before
// their RPC returns success.
func (c *LRUTTLCache) Invalidate(ctx context.Context, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[userID]; ok {
		c.removeLocked(el)
	}
}

// removeLocked removes el from both the LRU list and the index map.
// Callers must hold c.mu.
func (c *LRUTTLCache) removeLocked(el *list.Element) {
	e := el.Value.(*entry)
	c.order.Remove(el)
	delete(c.items, e.userID)
}
