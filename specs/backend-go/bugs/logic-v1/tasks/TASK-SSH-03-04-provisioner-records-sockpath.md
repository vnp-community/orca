# TASK-SSH-03-04: `HandshakeInfo` carries `SockPath`; `Provisioner.Provision` records it

**From Solution:** SOL-SSH-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`
**Depends on:** TASK-SSH-03-03
**Status:** `[ ]` TODO

---

## Context

TASK-SSH-03-06's `relaySSHReconnect` background loop needs to call
`reattach()` later, without re-resolving the `SshTarget` and without
knowing the socket path some other way — it needs `sockPath` threaded from
the original `Provision` call through to the persistent `*session`.
`devserveragent.HandshakeInfo` (session.go) is the existing free-form struct
both `adapter/agentwsserver` and `adapter/sshrelay` already populate after
their own handshake — the natural place for this one relay-ssh-specific
field.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`,
extend `HandshakeInfo`:

```go
type HandshakeInfo struct {
	Platform     string   `json:"platform"`
	Arch         string   `json:"arch"`
	NodeVersion  string   `json:"nodeVersion"`
	AgentVersion string   `json:"agentVersion"`
	SessionID    string   `json:"sessionId"`
	Capabilities []string `json:"capabilities"`
	// SockPath is relay-ssh mode's Unix socket path for the detached agent
	// process (see adapter/sshrelay's launch/reattach — SOL-SSH-03), cached
	// on *session so relaySSHReconnect can call reattach() again without
	// re-resolving the SshTarget or re-deploying. Empty for
	// relay-websocket/direct-websocket, which have no detached process.
	SockPath string `json:"-"`
}
```

In `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go`,
update `Provision` to pass `ctx` into `launch` (TASK-SSH-03-03 changed its
signature) and thread `sockPath` into the `HandshakeInfo` it returns:

```go
	transport, sockPath, err := launch(ctx, conn, remoteDir, devServer.ID)
	if err != nil {
		_ = conn.Close()
		return nil, devserveragent.HandshakeInfo{}, err
	}

	info, err := p.receiveHandshake(ctx, transport)
	if err != nil {
		_ = transport.Close("handshake failed")
		return nil, devserveragent.HandshakeInfo{}, err
	}
	info.SockPath = sockPath

	return transport, info, nil
```

`Client.getOrProvisionSession` (`client.go:186-213`) already calls
`sess.attachTransport(t, info)` with the `HandshakeInfo` `Provision`
returns — no change needed there, `SockPath` rides along automatically once
`session`'s `attachTransport` stores the full `HandshakeInfo` (confirm it
does; if `attachTransport` currently discards fields it doesn't need, widen
it to keep `HandshakeInfo` as-is on `*session`, e.g. a `s.handshakeInfo`
field already exists per session.go's struct — `mu`-guarded, set inside
`attachTransport`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/sshrelay/... -v
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -v
```

Expected: existing `TestProvision_*` tests still pass; a new assertion
confirms `Provision`'s returned `HandshakeInfo.SockPath` matches what
`launch` reported; `session.handshakeInfo.SockPath` is readable after
`attachTransport`.
