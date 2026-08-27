# TASK-SSH-04-05: SSH tunnel primitive — `Connection.Forward`

**From Solution:** SOL-SSH-04
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/sshconn/tunnel.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`sshconn.Connection` exposes only `RunCommand`/`NewSession`/`SFTPClient`
today (`connector.go:203-262`) — no tunnel primitive. Port forwarding is a
standard capability of the already-authenticated `*ssh.Client` `Connection`
wraps (`client.Dial("tcp", remoteAddr)` — the client dialing *through* the
existing SSH connection), unrelated to `SOL-SSH-03`'s JSON-RPC relay bridge
over the same connection's exec channels. No new agent-side capability
needed.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/adapter/sshconn/tunnel.go`:

```go
package sshconn

import (
	"fmt"
	"io"
	"net"
)

// Tunnel is a live local:remote forward — the "process/tunnel handle"
// domain.PortForward's doc comment names. Close stops the listener and
// every in-flight forwarded connection.
type Tunnel struct {
	listener net.Listener
	done     chan struct{}
}

// Forward opens a local TCP listener on 127.0.0.1:localPort and, for every
// accepted connection, dials remotePort on conn's target via the SSH
// connection's own direct-tcpip channel type (client.Dial), then copies
// bytes both directions until either side closes.
func (conn *Connection) Forward(localPort, remotePort int) (*Tunnel, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("sshconn: binding local port %d: %w", localPort, err)
	}
	t := &Tunnel{listener: listener, done: make(chan struct{})}
	go t.acceptLoop(conn, remotePort)
	return t, nil
}

func (t *Tunnel) acceptLoop(conn *Connection, remotePort int) {
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return // listener closed — Close() was called
		}
		go t.serveOne(conn, remotePort, local)
	}
}

func (t *Tunnel) serveOne(conn *Connection, remotePort int, local net.Conn) {
	remote, err := conn.client.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort))
	if err != nil {
		_ = local.Close()
		return
	}
	go func() {
		_, _ = io.Copy(remote, local)
		_ = remote.Close()
	}()
	_, _ = io.Copy(local, remote)
	_ = local.Close()
}

// Close stops accepting new connections and closes the listener. In-flight
// forwarded connections are closed as their io.Copy calls observe the
// listener-side sockets closing — no separate connection registry needed
// for a bounded, best-effort teardown.
func (t *Tunnel) Close() error {
	select {
	case <-t.done:
		return nil // already closed
	default:
		close(t.done)
	}
	return t.listener.Close()
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshconn/... -run TestTunnel -v
```

Expected new test (`tunnel_test.go`), integration-style against a local
fake SSH server with an echo listener on a "remote" port: a client
connecting to the tunnel's local port round-trips bytes through it;
`Close()` stops accepting and terminates in-flight copies without a
goroutine leak.
