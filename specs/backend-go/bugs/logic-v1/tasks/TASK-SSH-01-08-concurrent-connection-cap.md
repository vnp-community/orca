# TASK-SSH-01-08: Concurrent-connection cap per (tenant, host) — BR-SSH-04

**From Solution:** SOL-SSH-01
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshconn/pool.go` (new)
**Depends on:** TASK-SSH-01-06
**Status:** `[x] DONE — sshconn.Cap (pool.go) wired into Connector.Connect and main.go; TestCap_* pass`

---

## Context

BR-SSH-04 caps concurrent SSH connections at 10 per `(tenant, host)` pair;
no such guard exists in `sshconn` today. This mirrors
`infra-fleet-service.md` §8's "cap concurrent TerminalSessions per
connectionId" backpressure precedent — an in-process counter, not a
TCP-level limit.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/adapter/sshconn/pool.go`:

```go
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
```

Wire it into `Connect` (extending TASK-SSH-01-06's rewrite): at the top of
`Connect`, before resolving the jump chain:

```go
func (c *Connector) Connect(ctx context.Context, target domain.SshTarget) (*Connection, error) {
	if c.cap != nil {
		release, err := c.cap.Acquire(target.TenantID, target.Host)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	// ... existing jump-chain dial logic ...
}
```

Production wiring in `main.go`: construct one shared `sshconn.NewCap()` and
pass it to `sshconn.NewConnector(...)`; tests pass `nil` for "no cap".

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshconn/... -run TestCap -v
```

Expected: the 11th concurrent `Acquire` for the same `(tenant, host)` fails
immediately without dialing; `release()` frees the slot; two different
hosts/tenants don't share a counter.
