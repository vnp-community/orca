# TASK-AUTH-05-01: Migration — `audit_log` gains `target_type`/`target_id`/`metadata`/`ip_address`

**From Solution:** SOL-AUTH-05
**Priority:** P0 — every later task in this set depends on these columns
**Service:** `auth-service` (postgres migration)
**File:** `backend-go/services/auth-service/migrations/0004_audit_log_metadata.up.sql` (+ `.down.sql`)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`auth-service.md` §4 already specifies `AuditEntry` as `actorUserID`, `action`, `resourceType`, `resourceID`, structured `payload` — the current schema has only a single `target` column. `ip_address` is separately called out in both the TDD (`sessions.ip inet`) and the spec as a first-class column, not JSON metadata. This migration adds the split columns and backfills `target_type`/`target_id` from the existing `target`/`action` columns. `target` is kept, not dropped, during a transition window.

If TASK-AUTH-02-01 (`0003_sessions_last_seen_ip_ua`) has not landed yet in this branch, number this migration `0003_audit_log_metadata` instead of `0004_` — use whatever the next unused migration number actually is at execution time.

## Changes to make

Create `backend-go/services/auth-service/migrations/0004_audit_log_metadata.up.sql`:

```sql
ALTER TABLE auth.audit_log
  ADD COLUMN target_type TEXT,
  ADD COLUMN target_id   TEXT,
  ADD COLUMN metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN ip_address  INET;

-- Backfill: split the existing `target` column on the pre-existing
-- action-name convention (user.* actions target a user, session.* actions
-- target a session) — best-effort, historical rows may have target_type
-- left NULL where the action name doesn't map cleanly.
UPDATE auth.audit_log SET
  target_type = split_part(action, '.', 1),
  target_id   = target
WHERE target_type IS NULL;

CREATE INDEX IF NOT EXISTS idx_audit_log_action ON auth.audit_log (action);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON auth.audit_log (target_type, target_id);
```

Create `backend-go/services/auth-service/migrations/0004_audit_log_metadata.down.sql`:

```sql
DROP INDEX IF EXISTS auth.idx_audit_log_target;
DROP INDEX IF EXISTS auth.idx_audit_log_action;

ALTER TABLE auth.audit_log
  DROP COLUMN IF EXISTS ip_address,
  DROP COLUMN IF EXISTS metadata,
  DROP COLUMN IF EXISTS target_id,
  DROP COLUMN IF EXISTS target_type;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/auth-service/migrations/
```

Expected: the new `.up.sql`/`.down.sql` pair follows the existing numbering convention with no gap or collision; run against a local/test Postgres instance if available to confirm both directions apply cleanly and the backfill `UPDATE` runs without error against existing rows.
