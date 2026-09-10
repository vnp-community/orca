# TASK-FLEET-01-04: `SshTargetRepository.Upsert`/`GetByHostUser` + postgres implementation

**From Solution:** SOL-FLEET-01
**Priority:** P1
**Service:** `infra-fleet-service` (usecase port + postgres adapter)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`, `backend-go/services/infra-fleet-service/internal/adapter/postgres/ssh_target_store.go`
**Depends on:** TASK-FLEET-01-02 (unique index), TASK-FLEET-01-03 (domain fields)
**Status:** [x] DONE — added Upsert/GetByHostUser to SshTargetRepository port + SshTargetStore (in existing repository.go, not a new file); fake updated; `TestSshTargetStore_Upsert` (real testcontainers Postgres) passes. Pre-existing unrelated failures in TestRepository_ResolveConnection_FoundAndNotFound/TestRepository_RegisterAndGet_PersistsSSHTargetID confirmed present on baseline (not caused by this change).

---

## Context

The import usecase (TASK-FLEET-01-05) needs an upsert-by-`(tenant_id, host,
user_name)` primitive and a non-mutating existence probe for its dry-run
path — neither exists on `SshTargetRepository` today.

## Changes to make

`internal/usecase/ports.go`:

```go
type SshTargetRepository interface {
    Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
    Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
    List(ctx context.Context, tenantID string) ([]domain.SshTarget, error)
    // Upsert inserts or updates by (tenant_id, host, user_name) — the
    // conflict target the migration's unique index establishes.
    // updated=true means an existing row's vault_ssh_role/project/tags were
    // overwritten; updated=false means a new row was inserted.
    Upsert(ctx context.Context, target domain.SshTarget) (saved domain.SshTarget, updated bool, err error)
    // GetByHostUser is a narrow existence-probe used only by the
    // dry-run import path — it does not commit anything.
    GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error)
}
```

`internal/adapter/postgres/ssh_target_store.go`, add:

```go
func (s *SshTargetStore) Upsert(ctx context.Context, target domain.SshTarget) (domain.SshTarget, bool, error) {
    const query = `
        INSERT INTO infra.ssh_targets (id, tenant_id, host, user_name, vault_ssh_role, project, tags)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (tenant_id, host, user_name) DO UPDATE SET
          vault_ssh_role = EXCLUDED.vault_ssh_role,
          project = EXCLUDED.project,
          tags = EXCLUDED.tags
        RETURNING id, (xmax != 0) AS updated`
    var id string
    var updated bool
    if err := s.pool.QueryRow(ctx, query, target.ID, target.TenantID, target.Host, target.UserName, target.VaultSSHRole, target.Project, target.Tags).Scan(&id, &updated); err != nil {
        return domain.SshTarget{}, false, err
    }
    target.ID = id
    return target, updated, nil
}

func (s *SshTargetStore) GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error) {
    const query = `SELECT id, tenant_id, host, user_name, vault_ssh_role, project, tags
        FROM infra.ssh_targets WHERE tenant_id = $1 AND host = $2 AND user_name = $3`
    var t domain.SshTarget
    err := s.pool.QueryRow(ctx, query, tenantID, host, userName).Scan(&t.ID, &t.TenantID, &t.Host, &t.UserName, &t.VaultSSHRole, &t.Project, &t.Tags)
    if errors.Is(err, pgx.ErrNoRows) {
        return domain.SshTarget{}, false, nil
    }
    if err != nil {
        return domain.SshTarget{}, false, err
    }
    return t, true, nil
}
```

The `xmax != 0` trick is the standard Postgres idiom for "insert vs. update"
in one round trip — avoids a separate `SELECT ... FOR UPDATE` per row on a
bulk-import fan-in.

Also update any existing fake/mock implementing `SshTargetRepository` in
`internal/usecase/*_test.go` fakes to add these two methods.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestSshTargetStore_Upsert -v
```

Expected: `Upsert` twice with the same `(host,user)` and a changed
`vault_ssh_role` — second call returns `updated=true`, row count stays 1,
unique index enforces the conflict target.
