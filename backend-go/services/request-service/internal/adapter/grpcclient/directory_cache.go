package grpcclient

import (
	"sync"
	"time"
)

// directoryCache keeps directory answers for a short TTL so one approval burst does not become N tenant-service calls.
// Keys include the tenant, so entries never cross tenants.
type directoryCache struct {
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	items map[string]directoryEntry
}

type directoryEntry struct {
	values  []string
	expires time.Time
}

const defaultDirectoryTTL = 60 * time.Second

func newDirectoryCache(ttl time.Duration) *directoryCache {
	if ttl <= 0 {
		ttl = defaultDirectoryTTL
	}
	return &directoryCache{ttl: ttl, now: time.Now, items: map[string]directoryEntry{}}
}

func (c *directoryCache) get(key string) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok || !c.now().Before(e.expires) {
		delete(c.items, key)
		return nil, false
	}
	return append([]string(nil), e.values...), true
}

func (c *directoryCache) put(key string, values []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) > 4096 { // bound memory: drop everything rather than track LRU for a 60s cache
		c.items = map[string]directoryEntry{}
	}
	c.items[key] = directoryEntry{values: append([]string(nil), values...), expires: c.now().Add(c.ttl)}
}
