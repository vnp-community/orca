# TASK-FLEET-02-01: `DevServerStatus` + platform/version fields on `domain.DevServer`

**From Solution:** SOL-FLEET-02
**Priority:** P0
**Service:** `infra-fleet-service` (domain)
**File:** `backend-go/services/infra-fleet-service/internal/domain/dev_server.go`
**Depends on:** none
**Status:** [x] DONE — added DevServerStatus type + Status/Platform/Arch/NodeVersion/AgentVersion/LastProvisionedAt fields; NewDevServer defaults Status to Pending. Full service test suite passes.

---

## Context

`BulkProvisionFleet` (TASK-FLEET-02-05) needs a status field to persist
provisioning outcomes into — this closes BUG-FLEET-02's "no
`degraded`/`unhealthy` status field to persist into" gap. These fields are
shared with SOL-FLEET-04 (handshake-fact persistence), so only add them
once here.

## Changes to make

```go
// internal/domain/dev_server.go (extended)
type DevServerStatus string

const (
    DevServerStatusPending   DevServerStatus = "pending"   // registered, never provisioned
    DevServerStatusHealthy   DevServerStatus = "healthy"
    DevServerStatusDegraded  DevServerStatus = "degraded"  // prerequisites marginal or health degraded
    DevServerStatusUnhealthy DevServerStatus = "unhealthy" // provisioning/deploy failed after retries
)

type DevServer struct {
    ID, TenantID, Host string
    Mode                ConnectionMode
    SSHTargetID         string
    Status              DevServerStatus // new
    Platform, Arch, NodeVersion, AgentVersion string // new
    LastProvisionedAt   *time.Time // new
}
```

`NewDevServer`'s existing invariants are unchanged; `Status` defaults to
`DevServerStatusPending` when unset (registration via `devServer.add`/
`RegisterDevServer` doesn't provision, so `pending` is the honest initial
value).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/domain/... -run TestNewDevServer -v
```

Expected: `NewDevServer` unaffected; new fields default to zero
values/`pending`, no new invariant added.
