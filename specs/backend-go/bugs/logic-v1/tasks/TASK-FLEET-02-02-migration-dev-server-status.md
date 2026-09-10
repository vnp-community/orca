# TASK-FLEET-02-02: Migration — `status`/platform columns on `infra.dev_servers`

**From Solution:** SOL-FLEET-02
**Priority:** P0
**Service:** `infra-fleet-service` (postgres migration)
**File:** `backend-go/services/infra-fleet-service/migrations/0008_dev_server_status.up.sql` (new, plus matching `.down.sql`)
**Depends on:** none
**Status:** [x] DONE — added 0008 up/down migration (0007 already taken by TASK-FLEET-01-02); no duplicate numbering.

---

## Context

Correction from the SOL doc: it proposed `0007_dev_server_status.up.sql`,
but `0007` is now taken by SOL-FLEET-01's
`0007_ssh_target_project_tags.up.sql` (see TASK-FLEET-01-02) — use `0008`.
If TASK-FLEET-01-02 has not landed yet when this runs, re-check the highest
existing migration number in this directory before picking the file name.

## Changes to make

`backend-go/services/infra-fleet-service/migrations/0008_dev_server_status.up.sql`:

```sql
ALTER TABLE infra.dev_servers
  ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','healthy','degraded','unhealthy')),
  ADD COLUMN platform TEXT, ADD COLUMN arch TEXT,
  ADD COLUMN node_version TEXT, ADD COLUMN agent_version TEXT,
  ADD COLUMN last_provisioned_at TIMESTAMPTZ;
```

`backend-go/services/infra-fleet-service/migrations/0008_dev_server_status.down.sql`:

```sql
ALTER TABLE infra.dev_servers
  DROP COLUMN IF EXISTS last_provisioned_at,
  DROP COLUMN IF EXISTS agent_version,
  DROP COLUMN IF EXISTS node_version,
  DROP COLUMN IF EXISTS arch,
  DROP COLUMN IF EXISTS platform,
  DROP COLUMN IF EXISTS status;
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
ls migrations/ | sed -E 's/^([0-9]+)_.*/\1/' | sort | uniq -d  # no output = no numbering collision
go test ./internal/adapter/postgres/... -run TestDevServer -v
```

Expected: no duplicate migration-number prefixes; migration applies cleanly
in the adapter test suite (testcontainers Postgres).
