# TASK-FLEET-04-02: `DevServerAgentClient.LastHandshakeInfo`

**From Solution:** SOL-FLEET-04
**Priority:** P1
**Service:** `infra-fleet-service` (devserveragent adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`
**Depends on:** none
**Status:** [x] DONE — session.attachTransport already stored HandshakeInfo (task's premise about it possibly discarding it did not hold); added session.handshakeInfoSnapshot() + Client.LastHandshakeInfo(devServerID). Note: Client.mu is a plain sync.Mutex, not RWMutex — used Lock/Unlock, not RLock/RUnlock as the task's pseudocode assumed. Tests cover handshaked-session-returns-info and unknown-dev-server-returns-false; pass.

---

## Context

`EstablishConnection` (TASK-FLEET-04-03) needs to persist handshake-derived
facts (platform/arch/node version) after a successful connect, but
`Health()` only returns a bool today. `session.attachTransport` already
receives `HandshakeInfo` at connect time — it just wasn't retained anywhere
queryable. This adds a cheap in-memory lookup, not a new RPC.

## Changes to make

```go
// internal/adapter/devserveragent/client.go

// LastHandshakeInfo returns the HandshakeInfo captured at the most recent
// successful attachTransport for devServerID, if a live session exists.
// Cheap in-memory lookup — no round trip to the remote host.
func (c *Client) LastHandshakeInfo(devServerID string) (HandshakeInfo, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    session, ok := c.sessions[devServerID] // adjust to this file's actual session-registry field name
    if !ok || !session.isHandshaked() {
        return HandshakeInfo{}, false
    }
    return session.handshakeInfo, true
}
```

Retain `HandshakeInfo` on the session struct at `attachTransport` time if it
is not already stored there (check `session.go`'s `attachTransport`
implementation — it receives `HandshakeInfo` today per SOL-FLEET-04's
references at `client.go:211,234`, but may currently discard rather than
store it).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -run TestLastHandshakeInfo -v
```

Expected: a session that has completed handshake returns
`(info, true)` matching what `attachTransport` received; a session that
hasn't handshaked, or doesn't exist, returns `(HandshakeInfo{}, false)`.
