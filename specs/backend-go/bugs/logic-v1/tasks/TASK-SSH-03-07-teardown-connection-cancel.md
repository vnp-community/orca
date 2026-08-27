# TASK-SSH-03-07: `TeardownConnection` RPC — cancel an in-flight reconnect (BR-SSH-13)

**From Solution:** SOL-SSH-03
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** TASK-SSH-03-06, TASK-SSH-03-05
**Status:** `[ ]` TODO

---

## Context

`infra-fleet-service.md` §3 already specifies `TeardownConnection`, but the
current proto doesn't implement it — completing an already-designed RPC,
not inventing one. It's the user-facing "Cancel" action for BR-SSH-13's
"Reconnecting..." UX: stop `relaySSHReconnect`'s backoff loop and mark the
connection closed.

## Changes to make

`backend-go/proto/orca/infrafleet/v1/infrafleet.proto` — add the RPC (near
`EstablishConnection`) and its messages:

```protobuf
  rpc TeardownConnection(TeardownConnectionRequest) returns (google.protobuf.Empty);
```

```protobuf
message TeardownConnectionRequest {
  string connection_id = 1;
}
```

Regenerate: `buf generate proto` from `backend-go/`.

`backend-go/services/infra-fleet-service/internal/usecase/teardown_connection.go` (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TeardownConnectionInput struct {
	ConnectionID string
}

// TeardownConnection is BR-SSH-13's "Cancel" action: marks the connection
// row closed and stops any in-flight relaySSHReconnect backoff loop for its
// dev server — idempotent on an already-closed connection.
type TeardownConnection struct {
	conns ConnectionRepository
	agent DevServerAgentClient
}

func NewTeardownConnection(conns ConnectionRepository, agent DevServerAgentClient) *TeardownConnection {
	return &TeardownConnection{conns: conns, agent: agent}
}

func (uc *TeardownConnection) Execute(ctx context.Context, in TeardownConnectionInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	devServer, found, err := uc.conns.GetDevServerByConnection(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TEARDOWN_LOOKUP_FAILED", "failed to resolve connection", err)
	}
	if err := uc.conns.UpdateStatus(ctx, tenantID, in.ConnectionID, "closed"); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_TEARDOWN_FAILED", "failed to mark connection closed", err)
	}
	if found {
		uc.agent.CancelReconnect(devServer.ID) // no-op if no reconnect loop is running — see Client.CancelReconnect
	}
	return nil
}
```

Add `UpdateStatus(ctx context.Context, tenantID, connectionID, status string) error`
and `GetDevServerByConnection(ctx context.Context, tenantID, connectionID string) (domain.DevServer, bool, error)`
to `ConnectionRepository` in `ports.go` (implement both on
`postgres.Repository`, mirroring `GetActiveByDevServer`'s existing query
shape). Add `CancelReconnect(devServerID string)` to `DevServerAgentClient`
in `ports.go`.

`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` —
implement `CancelReconnect`:

```go
// CancelReconnect stops devServerID's session's relaySSHReconnect (or
// relay-websocket backgroundReconnect) loop immediately, mirroring
// session.close()'s existing closeCh-signaling shape — the session itself
// is not closed, only its in-flight reconnect attempt is abandoned, so a
// later Exec/Health call still triggers a fresh getOrProvisionSession/
// getOrDialSession rather than staying permanently dead.
func (c *Client) CancelReconnect(devServerID string) {
	c.mu.Lock()
	sess, ok := c.sessions[devServerID]
	c.mu.Unlock()
	if !ok {
		return
	}
	sess.cancelReconnect()
}
```

`session.go` — add `cancelReconnect`, reusing the existing `closeCh` signal
`relaySSHReconnect`/`backgroundReconnect` already select on (same pattern
`close()` uses, but without marking the session `closed`):

```go
// cancelReconnect unblocks a waiting backgroundReconnect/relaySSHReconnect
// loop without closing the session outright — TeardownConnection's cancel
// path. Safe to call when no reconnect loop is running (closeCh has no
// listener; the send below would need a listener to matter, but this
// signals via closing NOT sending, so re-closing an already-closed channel
// must be guarded — reuse closeMu the same way close() does).
func (s *session) cancelReconnect() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	select {
	case <-s.closeCh:
		return // already closed/cancelled
	default:
		close(s.closeCh)
		s.mu.Lock()
		s.closeCh = make(chan struct{}) // replace so a FUTURE reconnect loop (next drop) gets a fresh channel
		s.mu.Unlock()
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./...
go test ./services/infra-fleet-service/internal/usecase/... -run TestTeardownConnection -v
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -run TestSession_CancelReconnect -v
```

Expected: `TeardownConnection` marks the row closed and cancels an in-flight
`relaySSHReconnect` loop (fake `closeCh`-equivalent signal observed);
idempotent on an already-closed connection (`GetDevServerByConnection`
returning `found=false` skips `CancelReconnect` without erroring).
