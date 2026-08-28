# TASK-SSH-04-03: `domain.PortForward` entity + `process_name` migration

**From Solution:** SOL-SSH-04
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/port_forward.go` (new)
**Depends on:** none
**Status:** `[x] DONE — domain.PortForward + migration 0008 + PortForwardRepository port + postgres.PortForwardStore added`

---

## Context

`infra.port_forwards` already exists (`migrations/0002_connections.up.sql:36-44`:
`id, tenant_id, connection_id, local_port, remote_port, created_at`) but is
schema-only — no domain type, no usecase reads/writes it yet. This task adds
the domain entity per `infra-fleet-service.md` §4, plus one column beyond
the existing DDL: `process_name`, carrying `ports.detect`'s `processName`
through to the frontend's "Port 3001 → remote:3000 (node)" notification.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/domain/port_forward.go`:

```go
package domain

// PortForwardStatus is a live local:remote tunnel's lifecycle state.
type PortForwardStatus string

const (
	PortForwardStatusActive PortForwardStatus = "active"
	PortForwardStatusClosed PortForwardStatus = "closed"
)

// PortForward is a live local:remote tunnel, per
// specs/backend-go/services/infra-fleet-service.md §4/§5.
type PortForward struct {
	ID           string
	TenantID     string
	ConnectionID string
	LocalPort    int
	RemotePort   int
	ProcessName  string // carries ports.detect's processName through to the
	                    // frontend notification — additive beyond §5's DDL
	Status PortForwardStatus
}
```

Create `backend-go/services/infra-fleet-service/migrations/0008_port_forwards_process_status.up.sql`:

```sql
-- infra.port_forwards was schema-only (§5's DDL) until SOL-SSH-04 gave it a
-- real writer (usecase.PollWorkspacePorts). process_name/status are the two
-- columns the domain entity needs beyond the original DDL.
ALTER TABLE infra.port_forwards ADD COLUMN process_name TEXT NOT NULL DEFAULT '';
ALTER TABLE infra.port_forwards ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
```

`0008_port_forwards_process_status.down.sql`:

```sql
ALTER TABLE infra.port_forwards DROP COLUMN status;
ALTER TABLE infra.port_forwards DROP COLUMN process_name;
```

Add a `PortForwardRepository` port to
`backend-go/services/infra-fleet-service/internal/usecase/ports.go`:

```go
type PortForwardRepository interface {
	Create(ctx context.Context, pf domain.PortForward) (domain.PortForward, error)
	UpdateStatus(ctx context.Context, tenantID, id string, status domain.PortForwardStatus) error
	ListActiveByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.PortForward, error)
}
```

Implement it in
`backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go`
as a new `PortForwardStore` type (same pool-sharing pattern
`SshTargetStore` already uses), with `Create`/`UpdateStatus`/
`ListActiveByConnection` querying `infra.port_forwards`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" down 1
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestPortForward -v
```

Expected: migration round-trips cleanly; `PortForwardStore.Create` then
`ListActiveByConnection` round-trips `ProcessName`/`Status`.
