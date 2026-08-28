# TASK-SSH-03-05: `"reconnecting"` connection status + `GetSshState` surfaces it (BR-SSH-13)

**From Solution:** SOL-SSH-03
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/connection.go`
**Depends on:** none
**Status:** `[x] DONE — domain.Connection.Status/usecase.SshState/proto GetSshStateResponse carry "reconnecting"; TestGetSshState_ReconnectingStatus passes`

---

## Context

`domain.Connection.Status` today is `"established" | "degraded" | "closed"`
(`connection.go:31`); `SshState` (`get_ssh_state.go:15-19`) only reports a
`Connected bool`, not enough to render the spec's "Reconnecting..." overlay
UX while `relaySSHReconnect` (TASK-SSH-03-06) is mid-backoff.

## Changes to make

`backend-go/services/infra-fleet-service/internal/domain/connection.go` —
update the comment documenting the `Status` field's valid values:

```go
	// Status/LastActivityAt are set by EstablishConnection (ssh.connect,
	// SOL-024/TASK-164) — empty/nil for connections predating this field
	// (worktree-bound connections created via CreateConnection, not
	// EstablishConnection). See migrations/0004_connection_status.
	Status         string // "established" | "degraded" | "reconnecting" | "closed"
	LastActivityAt *time.Time
```

No migration needed — `Status` is already a free-form `TEXT` column; this
is purely a new valid value, not a schema change.

`backend-go/services/infra-fleet-service/internal/usecase/get_ssh_state.go`
— widen `SshState` to carry the actual status string, not just a bool:

```go
type SshState struct {
	Connected    bool
	Status       string // "" | "established" | "degraded" | "reconnecting" | "closed"
	ConnectionID string
	LastActivity *time.Time
}

func (uc *GetSshState) Execute(ctx context.Context, in SshStateInput) (SshState, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return SshState{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, in.SshTargetID)
	if err != nil || !found {
		return SshState{Connected: false}, err
	}
	conn, found, err := uc.conns.GetActiveByDevServer(ctx, tenantID, devServer.ID)
	if err != nil || !found {
		return SshState{Connected: false}, err
	}
	return SshState{
		Connected:    conn.Status != "reconnecting" && conn.Status != "closed",
		Status:       conn.Status,
		ConnectionID: conn.ID,
		LastActivity: conn.LastActivityAt,
	}, nil
}
```

Add `Status`/`ReconnectingUnixMs`-equivalent fields to
`GetSshStateResponse` in `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
(currently `connected`/`connection_id`/`last_activity_unix_ms`):

```protobuf
message GetSshStateResponse {
  bool connected = 1;
  string connection_id = 2;
  int64 last_activity_unix_ms = 3;
  string status = 4; // "" | "established" | "degraded" | "reconnecting" | "closed"
}
```

Regenerate stubs (`buf generate proto` from `backend-go/`) and update the
gRPC server's `GetSshState` mapping to populate the new field.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./...
go test ./services/infra-fleet-service/internal/usecase/... -run TestGetSshState -v
```

Expected: build clean; `GetSshState` reports `Status: "reconnecting"` and
`Connected: false` for a connection row in that state, distinct from
`"closed"`.
