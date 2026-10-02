package mcpserver

import (
	"sync"
	"time"
)

// readySessions remembers which sessions of THIS replica completed the
// initialize -> notifications/initialized handshake. The SDK exposes no
// accessor for that state, and the session itself lives in SDK memory too, so
// the two share a lifetime: entries idle for 2x the session TTL are pruned.
// BE-MCP-SOL-004 replaces both with a durable SessionStore.
type readySessions struct {
	mu    sync.Mutex
	ttl   time.Duration
	seen  map[string]time.Time
	ops   int
	clock func() time.Time
}

func newReadySessions(idleTTL time.Duration) *readySessions {
	return &readySessions{ttl: 2 * idleTTL, seen: map[string]time.Time{}, clock: time.Now}
}

func (r *readySessions) mark(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen[id] = r.clock()
	if r.ops++; r.ops%256 == 0 {
		cutoff := r.clock().Add(-r.ttl)
		for k, t := range r.seen {
			if t.Before(cutoff) {
				delete(r.seen, k)
			}
		}
	}
}

func (r *readySessions) has(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.seen[id]; !ok {
		return false
	}
	r.seen[id] = r.clock() // activity keeps the entry as long as the SDK keeps the session
	return true
}
