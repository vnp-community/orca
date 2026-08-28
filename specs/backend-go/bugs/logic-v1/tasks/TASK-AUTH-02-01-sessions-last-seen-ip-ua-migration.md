# TASK-AUTH-02-01: Migration — `sessions` gains `last_seen_at`/`ip`/`user_agent`

**From Solution:** SOL-AUTH-02
**Priority:** P0 — every later task in this set reads/writes these columns
**Service:** `auth-service` (postgres migration)
**File:** `backend-go/services/auth-service/migrations/0003_sessions_last_seen_ip_ua.up.sql` (+ `.down.sql`)
**Depends on:** none
**Status:** `[x]` DONE — added `0003_sessions_last_seen_ip_ua.{up,down}.sql` following the `0001`/`0002` naming convention; no local Postgres available to apply, SQL reviewed for correctness.

---

## Context

`auth-service.md` §4/§5 already specify `Session.lastSeenAt`/`.ip`/`.userAgent` and matching `sessions` table columns, but the migrations directory (`0001_init`, `0002_access_policies`) never added them. This task adds the columns and the reaper's scan index.

## Changes to make

Create `backend-go/services/auth-service/migrations/0003_sessions_last_seen_ip_ua.up.sql`:

```sql
ALTER TABLE auth.sessions
  ADD COLUMN last_seen_at TIMESTAMPTZ,
  ADD COLUMN ip           INET,
  ADD COLUMN user_agent   TEXT;

-- Index the reaper's scan predicate.
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON auth.sessions (expires_at);
```

`last_seen_at` starts `NULL` (never touched since creation) — this is intentional, distinct from "touched at creation".

Create `backend-go/services/auth-service/migrations/0003_sessions_last_seen_ip_ua.down.sql`:

```sql
DROP INDEX IF EXISTS auth.idx_sessions_expires_at;

ALTER TABLE auth.sessions
  DROP COLUMN IF EXISTS last_seen_at,
  DROP COLUMN IF EXISTS ip,
  DROP COLUMN IF EXISTS user_agent;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/auth-service/migrations/
# confirm the migration tool used by this service picks it up (check
# cmd/server/main.go or a migrate Makefile target for the exact invocation)
grep -rn "migrate" services/auth-service/cmd/server/main.go
```

Expected: the new `.up.sql`/`.down.sql` pair sits alongside `0001_init`/`0002_access_policies` and follows the same naming/numbering convention; run against a local/test Postgres instance if one is available to confirm both directions apply cleanly.
