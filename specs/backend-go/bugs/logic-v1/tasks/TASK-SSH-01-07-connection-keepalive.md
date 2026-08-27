# TASK-SSH-01-07: `Connection` keepalive loop (BR-SSH-03)

**From Solution:** SOL-SSH-01
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go`
**Depends on:** TASK-SSH-01-06
**Status:** `[ ]` TODO

---

## Context

The raw `*ssh.Client` `sshconn.Connection` wraps has no keepalive today —
`devserveragent`'s session-level keepalive (`session.go`'s `keepAliveLoop`)
is a JSON-RPC frame one layer above this, unrelated. BR-SSH-03 wants a plain
SSH transport keepalive so a silently-dropped TCP connection is detected
promptly rather than hanging until the next real request times out.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go`,
extend `Connection`:

```go
// Connection wraps a live, authenticated SSH connection to one target.
type Connection struct {
	client   *ssh.Client
	closeCh  chan struct{}
	closeOne sync.Once
}

// keepAlive sends an SSH keepalive@openssh.com global request every interval
// until ctx is cancelled or the connection is closed — matches the spec's
// ServerAliveInterval (30s). A missed write means the connection is dead;
// the caller (sshrelay.Provisioner, right after Connect succeeds) starting
// this loop is what feeds a drop into BUG-SSH-03's reconnect detection
// (SOL-SSH-03), same "who starts it" placement as
// devserveragent/session.go's keepAliveLoop, one layer lower.
func (conn *Connection) keepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, _, err := conn.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return // caller's next operation on this Connection will observe the same failure
			}
		case <-ctx.Done():
			return
		case <-conn.closeCh:
			return
		}
	}
}

// StartKeepAlive launches keepAlive in a goroutine — separated from
// Connect() itself so a caller that doesn't want the loop (e.g. a
// short-lived probe connection) can opt out.
func (conn *Connection) StartKeepAlive(ctx context.Context, interval time.Duration) {
	go conn.keepAlive(ctx, interval)
}
```

Initialize `closeCh` in `Connect`'s `return &Connection{...}` (TASK-SSH-01-06):
`&Connection{client: current, closeCh: make(chan struct{})}`.

Update `Close()`:

```go
func (conn *Connection) Close() error {
	conn.closeOne.Do(func() { close(conn.closeCh) })
	return conn.client.Close()
}
```

Wire the caller: in `sshrelay.Provisioner.Provision`, right after
`conn, err := p.connector.Connect(ctx, target)` succeeds, call
`conn.StartKeepAlive(context.Background(), 30*time.Second)` (a
`Provisioner`-lifetime context, not the request `ctx`, since the connection
outlives the `Provision` call).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshconn/... -run TestKeepAlive -v
```

Expected: a test against a fake SSH server asserts `keepalive@openssh.com`
requests arrive at the configured interval; `Close()` stops the loop without
a goroutine leak (assert via `goleak` or a closed-channel check).
