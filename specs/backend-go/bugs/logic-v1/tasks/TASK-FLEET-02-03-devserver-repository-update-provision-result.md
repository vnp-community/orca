# TASK-FLEET-02-03: `DevServerRepository.UpdateProvisionResult`/`FindBySshTarget`

**From Solution:** SOL-FLEET-02
**Priority:** P1
**Service:** `infra-fleet-service` (usecase port + postgres adapter)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`, `backend-go/services/infra-fleet-service/internal/adapter/postgres/dev_server_store.go`
**Depends on:** TASK-FLEET-02-01, TASK-FLEET-02-02
**Status:** `[ ]` TODO

---

## Context

`bulkProvisionOne` (TASK-FLEET-02-05) needs to look up an existing
`DevServer` for an `SshTarget` (find-or-create) and persist the outcome of
one provisioning attempt.

## Changes to make

`internal/usecase/ports.go`:

```go
type DevServerRepository interface {
    // ... existing methods unchanged ...

    // FindBySshTarget looks up the DevServer registered against a given
    // SshTarget, if any (find-or-create support for bulk provisioning).
    FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error)

    // UpdateProvisionResult persists the outcome of one provisioning
    // attempt — status plus the handshake facts SOL-FLEET-04 needs
    // surfaced. Called once per server at the end of bulkProvisionOne,
    // success or failure.
    UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerStatus, info devserveragent.HandshakeInfo, provisionedAt time.Time) error
}
```

Postgres implementation: `FindBySshTarget` is a `SELECT ... WHERE tenant_id
= $1 AND ssh_target_id = $2`, `false` on `pgx.ErrNoRows`.
`UpdateProvisionResult` is an `UPDATE infra.dev_servers SET status = $3,
platform = $4, arch = $5, node_version = $6, agent_version = $7,
last_provisioned_at = $8 WHERE tenant_id = $1 AND id = $2`, mapping
`info.Platform`/`info.Arch`/`info.NodeVersion`/`info.AgentVersion` from
`devserveragent.HandshakeInfo`.

Update any existing fake implementing `DevServerRepository` in
`internal/usecase/*_test.go` fakes to add both new methods.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestDevServerStore_UpdateProvisionResult -v
```

Expected: `UpdateProvisionResult` persists `status`/`platform`/
`node_version`/`last_provisioned_at`; a second call updates in place
(idempotent), no duplicate row.
