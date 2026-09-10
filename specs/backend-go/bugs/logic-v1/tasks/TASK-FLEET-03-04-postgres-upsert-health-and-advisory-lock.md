# TASK-FLEET-03-04: Postgres `UpsertFleetHealth`/`GetPrevious`/`TryLock` + `ListAllForPolling`

**From Solution:** SOL-FLEET-03
**Priority:** P1
**Service:** `infra-fleet-service` (postgres adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-FLEET-03-02, TASK-FLEET-03-03
**Status:** [x] DONE — ListAllForPolling was already implemented in TASK-FLEET-03-03's pass (with intermediate string vars for the mode/status scan, consistent with this file's existing Get/List convention). Added UpsertFleetHealth/GetPrevious/TryLock. Real testcontainers-Postgres tests: TestUpsertFleetHealthAndGetPrevious (upsert-by-PK round trip), TestTryLock_MutualExclusionAndReleaseAllowsReacquire (exactly-one-of-two-concurrent-locks-succeeds, reacquire after unlock), TestListAllForPolling_IsCrossTenant all pass. Pre-existing flaky/order-dependent failures (TestRepository_ResolveConnection_FoundAndNotFound, TestRepository_List_FiltersByTenant under container-count contention) confirmed unrelated — pass individually.

---

## Context

Postgres-backed implementations of the ports defined in TASK-FLEET-03-03.
`TryLock` uses a held connection (not the pool) since advisory locks are
session-scoped.

## Changes to make

```go
// internal/adapter/postgres/repository.go

func (r *Repository) UpsertFleetHealth(ctx context.Context, sample domain.DevServerHealth) error {
    const query = `
        INSERT INTO infra.fleet_health (dev_server_id, reachable, cpu_percent, ram_percent, disk_percent, latency_ms, status, checked_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, now())
        ON CONFLICT (dev_server_id) DO UPDATE SET
          reachable = EXCLUDED.reachable, cpu_percent = EXCLUDED.cpu_percent,
          ram_percent = EXCLUDED.ram_percent, disk_percent = EXCLUDED.disk_percent,
          latency_ms = EXCLUDED.latency_ms, status = EXCLUDED.status, checked_at = now()`
    _, err := r.pool.Exec(ctx, query, sample.DevServerID, sample.Reachable, sample.CPUPercent, sample.RAMPercent, sample.DiskPercent, sample.LatencyMS, sample.Status)
    return err
}

func (r *Repository) GetPrevious(ctx context.Context, devServerID string) (domain.DevServerHealth, bool, error) {
    const query = `SELECT dev_server_id, reachable, cpu_percent, ram_percent, disk_percent, latency_ms, status
        FROM infra.fleet_health WHERE dev_server_id = $1`
    var h domain.DevServerHealth
    err := r.pool.QueryRow(ctx, query, devServerID).Scan(&h.DevServerID, &h.Reachable, &h.CPUPercent, &h.RAMPercent, &h.DiskPercent, &h.LatencyMS, &h.Status)
    if errors.Is(err, pgx.ErrNoRows) {
        return domain.DevServerHealth{}, false, nil
    }
    if err != nil {
        return domain.DevServerHealth{}, false, err
    }
    return h, true, nil
}

func (r *Repository) TryLock(ctx context.Context, devServerID string) (bool, func(), error) {
    conn, err := r.pool.Acquire(ctx)
    if err != nil {
        return false, nil, err
    }
    var locked bool
    if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, devServerID).Scan(&locked); err != nil {
        conn.Release()
        return false, nil, err
    }
    if !locked {
        conn.Release()
        return false, nil, nil
    }
    unlock := func() {
        _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, devServerID)
        conn.Release()
    }
    return true, unlock, nil
}

// ListAllForPolling is cross-tenant by design — no tenant_id filter.
func (r *Repository) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
    const query = `SELECT id, tenant_id, host, mode, ssh_target_id, status, platform, arch, node_version, agent_version FROM infra.dev_servers`
    rows, err := r.pool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var servers []domain.DevServer
    for rows.Next() {
        var ds domain.DevServer
        if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &ds.Mode, &ds.SSHTargetID, &ds.Status, &ds.Platform, &ds.Arch, &ds.NodeVersion, &ds.AgentVersion); err != nil {
            return nil, err
        }
        servers = append(servers, ds)
    }
    return servers, rows.Err()
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run 'TestUpsertFleetHealth|TestTryLock|TestListAllForPolling' -v
```

Expected (testcontainers Postgres): two concurrent `TryLock` calls for the
same `devServerID` from two separate connections — exactly one succeeds;
after `unlock()`, a subsequent `TryLock` succeeds again.
