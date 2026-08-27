# TASK-SSH-03-06: `relaySSHReconnect` background loop — real reconnect for relay-ssh sessions

**From Solution:** SOL-SSH-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`
**Depends on:** TASK-SSH-03-04
**Status:** `[x] DONE — Provisioner.Reattach, devserveragent managedMode/SshReattacher/relaySSHReconnect, Client.WithRelaySSH wiring; TestSession_RelaySSHReconnect_* pass`

---

## Context

`backgroundReconnect` (`session.go:420-429`) no-ops entirely for relay-ssh
via the `managedExternally` bool — its own doc comment says "reconnecting
means redeploying+relaunching, not dialing this same transport again". With
the detached process (TASK-SSH-03-01/03) actually surviving an SSH drop,
that's no longer true: a dropped relay-ssh session should try
`reattach()` first (cheap, no redeploy) and only fall back to a full
`Provision` if the detached process itself is gone.

## Changes to make

**1. `adapter/sshrelay`** — add a `Reattach` method to `Provisioner`
(`provisioner.go`), the production implementation of the new port below:

```go
// Reattach re-dials devServer's SshTarget and bridges onto the already-
// running detached relay process at sockPath — the cheap path
// relaySSHReconnect takes on every retry after the first. Returns
// sshrelay.ErrDetachedProcessGone (wrapped) when the detached process
// itself is no longer alive, the caller's cue to fall back to a full
// Provision instead.
func (p *Provisioner) Reattach(ctx context.Context, devServer domain.DevServer, sockPath string) (devserveragent.Transport, error) {
	target, err := p.resolver.Get(ctx, devServer.TenantID, devServer.SSHTargetID)
	if err != nil {
		return nil, fmt.Errorf("sshrelay: resolving ssh target %q: %w", devServer.SSHTargetID, err)
	}
	conn, err := p.connector.Connect(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("sshrelay: dialing ssh target %q: %w", devServer.SSHTargetID, err)
	}
	transport, _, err := reattach(ctx, conn, remoteDir, sockPath)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return transport, nil
}
```

**2. `adapter/devserveragent`** — new port + `session` fields, in
`session.go`:

```go
// SshReattacher is relay-ssh's background-reconnect port, mirroring
// SshProvisioner's shape — implemented by adapter/sshrelay.Provisioner.
type SshReattacher interface {
	Reattach(ctx context.Context, devServer domain.DevServer, sockPath string) (Transport, error)
}

// managedMode replaces the old managedExternally bool — direct-websocket
// keeps the true no-op (inboundOnly), relay-ssh gets a real reconnect loop
// (relaySSHReattach) instead of a no-op.
type managedMode int

const (
	managedModeNone managedMode = iota // relay-websocket: backgroundReconnect dials as before
	managedModeInboundOnly              // direct-websocket: agent re-dials on its own
	managedModeRelaySSHReattach         // relay-ssh: relaySSHReconnect below
)
```

Replace `session`'s `managedExternally bool` field with
`managedMode managedMode`, and add:

```go
	// relaySSHDevServer/reattacher are set only for managedModeRelaySSHReattach
	// sessions (by Client.getOrProvisionSession) — relaySSHReconnect's inputs.
	relaySSHDevServer domain.DevServer
	reattacher        SshReattacher
```

(Add `"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"`
to session.go's imports.)

**3.** Replace `backgroundReconnect`'s early-return branch and add the new
loop:

```go
func (s *session) backgroundReconnect() {
	s.mu.Lock()
	mode := s.managedMode
	s.mu.Unlock()
	switch mode {
	case managedModeInboundOnly:
		return // agent must dial in again — see AttachInboundSession
	case managedModeRelaySSHReattach:
		s.relaySSHReconnect()
		return
	}
	// ... existing relay-websocket loop body, unchanged ...
}

// relaySSHReconnect mirrors backgroundReconnect's loop structure exactly
// (same backoffDelay call, same closed/superseded checks) but calls
// reattacher.Reattach instead of connect() — the detached agent process's
// in-memory state (its AgentSession, its pty-daemon children) survived the
// SSH drop untouched, only the bridge died, so no fresh agent.handshake is
// needed: this just confirms the bridge is live and reuses the
// HandshakeInfo captured at first Provision (cached via s.handshakeInfo).
func (s *session) relaySSHReconnect() {
	for {
		s.mu.Lock()
		alreadyLive := s.handshaked && s.transport != nil
		closed := s.closed
		attempt := s.reconnectAttempt
		sockPath := s.handshakeInfo.SockPath
		devServer := s.relaySSHDevServer
		reattacher := s.reattacher
		s.mu.Unlock()
		if alreadyLive || closed {
			return
		}

		delay := backoffDelay(s.cfg, attempt)
		select {
		case <-time.After(delay):
		case <-s.closeCh:
			return
		}

		s.mu.Lock()
		if s.closed || (s.handshaked && s.transport != nil) {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.DialTimeout+s.cfg.HandshakeTimeout)
		conn, err := reattacher.Reattach(ctx, devServer, sockPath)
		cancel()

		if errors.Is(err, sshrelayErrDetachedProcessGone) {
			// Detached process itself is gone — reattach can never succeed
			// again; the next Exec/Health call's getOrProvisionSession will
			// do a full re-Provision (redeploy+relaunch) once this loop exits.
			return
		}

		s.mu.Lock()
		if err != nil {
			s.reconnectAttempt++
		} else {
			s.reconnectAttempt = 0
		}
		s.mu.Unlock()

		if err == nil {
			s.attachTransport(conn, s.handshakeInfo) // reuse cached info, only transport/liveness changed
			return
		}
		if s.logger != nil {
			s.logger.Warn("devserveragent: relay-ssh reattach attempt failed", slog.String("host", s.host), slog.Int("attempt", attempt), slog.Any("error", err))
		}
	}
}
```

Note: reference `sshrelayErrDetachedProcessGone` via a small package-level
`var` in `devserveragent` that the `adapter/sshrelay` package's error is
compared against using `errors.Is` — since `devserveragent` must not import
`adapter/sshrelay` (wrong dependency direction per this codebase's
Dependency Inversion convention), have `SshReattacher.Reattach` wrap
`sshrelay.ErrDetachedProcessGone` in a `devserveragent`-local sentinel
(e.g. `ErrRelayDetachedProcessGone`) at the adapter boundary — `Provisioner.Reattach`'s
`fmt.Errorf("%w: %w", devserveragent.ErrRelayDetachedProcessGone, err)`
(Go 1.20+ multi-`%w` wrapping) instead of returning the raw `sshrelay` error.

**4.** `Client.getOrProvisionSession` (`client.go:186-213`) sets the new
fields instead of the old bool:

```go
	sess.managedMode = managedModeRelaySSHReattach
	sess.relaySSHDevServer = devServer
	sess.reattacher = c.sshReattacher // set via a new WithRelaySSH param — Provisioner implements both SshProvisioner and SshReattacher
```

`WithRelaySSH`'s signature widens to also store the same `*sshrelay.Provisioner`
value as `Client.sshReattacher SshReattacher` (it already implements both
interfaces after this task's step 1).

`getInboundSession`'s corresponding session gets `managedMode = managedModeInboundOnly`
in `AttachInboundSession` (replacing its `managedExternally = true` set).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -v
```

Expected new test `TestSession_RelaySSHReconnect_ReattachesWithoutRedeploy`
(mirrors `TestSession_BackgroundReconnect_RecoversAfterDropWithoutCallerRetry`):
asserts `Reattach` was called, `Provision` was not, for a live-detached-
process scenario; a second test asserts the `ErrRelayDetachedProcessGone`
path returns without looping further (leaving the session non-live for the
next caller's full re-Provision).
