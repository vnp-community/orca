# TASK-FLEET-03-02: Migration — `status` column on `infra.fleet_health`

**From Solution:** SOL-FLEET-03
**Priority:** P0
**Service:** `infra-fleet-service` (postgres migration)
**File:** `backend-go/services/infra-fleet-service/migrations/0009_fleet_health_status.up.sql` (new, plus matching `.down.sql`)
**Depends on:** none
**Status:** [x] DONE — added 0009 up/down migration (0008 taken by TASK-FLEET-02-02); no duplicate numbering. infra.fleet_health.dev_server_id is already the PK, confirmed usable as the upsert conflict target for TASK-FLEET-03-04.

---

## Context

Correction from the SOL doc: it proposed `0008_fleet_health_status.up.sql`,
but `0008` is now taken by SOL-FLEET-02's `0008_dev_server_status.up.sql`
(see TASK-FLEET-02-02) — use `0009`. If prior FLEET migrations have not
landed yet when this runs, re-check the highest existing migration number
in this directory before picking the file name.

## Changes to make

`backend-go/services/infra-fleet-service/migrations/0009_fleet_health_status.up.sql`:

```sql
ALTER TABLE infra.fleet_health
  ADD COLUMN status TEXT NOT NULL DEFAULT 'unreachable'
    CHECK (status IN ('healthy','degraded','unhealthy','unreachable'));
```

`backend-go/services/infra-fleet-service/migrations/0009_fleet_health_status.down.sql`:

```sql
ALTER TABLE infra.fleet_health DROP COLUMN IF EXISTS status;
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
ls migrations/ | sed -E 's/^([0-9]+)_.*/\1/' | sort | uniq -d  # no output = no numbering collision
go test ./internal/adapter/postgres/... -run TestFleetHealth -v
```
