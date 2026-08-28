# TASK-FLEET-01-02: Migration — `project`/`tags` columns + upsert unique index on `infra.ssh_targets`

**From Solution:** SOL-FLEET-01
**Priority:** P0
**Service:** `infra-fleet-service` (postgres migration)
**File:** `backend-go/services/infra-fleet-service/migrations/0007_ssh_target_project_tags.up.sql` (new, plus matching `.down.sql`)
**Depends on:** none
**Status:** [x] DONE — added 0007 up/down migration; no duplicate numbering (0007 free).

---

## Context

`Upsert`-by-`(host, user_name)` (TASK-FLEET-01-04) needs a uniqueness
constraint that does not exist today. Correction from the SOL doc: it
proposed `0006_ssh_target_project_tags.up.sql`, but `0006` is already taken
by `0006_browser_profiles.up.sql` in this service's migrations directory —
use `0007` instead.

## Changes to make

`backend-go/services/infra-fleet-service/migrations/0007_ssh_target_project_tags.up.sql`:

```sql
ALTER TABLE infra.ssh_targets
  ADD COLUMN project TEXT NOT NULL DEFAULT '',
  ADD COLUMN tags     TEXT[] NOT NULL DEFAULT '{}';

-- Upsert-by-hostname+user (BL-FLEET-01's "INSERT OR UPDATE by hostname+user")
-- needs this uniqueness constraint to exist at all — it does not today.
CREATE UNIQUE INDEX idx_infra_ssh_targets_tenant_host_user
  ON infra.ssh_targets (tenant_id, host, user_name);
```

`backend-go/services/infra-fleet-service/migrations/0007_ssh_target_project_tags.down.sql`:

```sql
DROP INDEX IF EXISTS idx_infra_ssh_targets_tenant_host_user;
ALTER TABLE infra.ssh_targets
  DROP COLUMN IF EXISTS tags,
  DROP COLUMN IF EXISTS project;
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
ls migrations/ | sort | uniq -c -w4 | awk '$1>2 {print "DUPLICATE PREFIX:", $0}'  # no output = no numbering collision
go test ./internal/adapter/postgres/... -run TestSshTarget -v
```

Expected: no duplicate migration-number prefixes; migration applies cleanly
against a fresh testcontainers Postgres in the adapter test suite.
