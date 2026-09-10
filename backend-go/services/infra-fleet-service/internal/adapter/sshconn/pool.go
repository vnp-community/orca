package sshconn

import (
	"fmt"
	"sync"
)

const maxConcurrentConnectionsPerHost = 10

// ErrTooManyConcurrentConnections is returned by Cap.Acquire when the
// (tenantID, host) pair is already at maxConcurrentConnectionsPerHost.
var ErrTooManyConcurrentConnections = fmt.Errorf("sshconn: too many concurrent connections to this host (max %d)", maxConcurrentConnectionsPerHost)

// Cap tracks in-flight connection attempts + live connections per
// (tenantID, host) pair, rejecting the 11th before ever dialing.
type Cap struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewCap() *Cap {
	return &Cap{counts: make(map[string]int)}
}

func capKey(tenantID, host string) string {
	return tenantID + "/" + host
}

// Acquire increments (tenantID, host)'s count, returning a release closure
// the caller MUST call on every return path (success and failure alike — a
// failed dial still occupied a slot briefly). Returns
// ErrTooManyConcurrentConnections without incrementing if already at cap.
func (c *Cap) Acquire(tenantID, host string) (release func(), err error) {
	key := capKey(tenantID, host)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts[key] >= maxConcurrentConnectionsPerHost {
		return nil, ErrTooManyConcurrentConnections
	}
	c.counts[key]++
	released := false
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if released {
			return
		}
		released = true
		c.counts[key]--
		if c.counts[key] <= 0 {
			delete(c.counts, key)
		}
	}, nil
}
