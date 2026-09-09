# TASK-TG-001-01: Migration — widen `task.tasks`, add `owner_id`/`share_token`-ready columns (additive-only)

**From Solution:** BE-SOL-001
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/migrations/0004_task_fields_and_comments.up.sql` (new), `backend-go/services/task-service/migrations/0004_task_fields_and_comments.down.sql` (new)
**Depends on:** None (first BE-SOL-001 task)
**Status:** `[ ]` TODO

---

## Context

Verified against the real, current migrations directory
(`ls backend-go/services/task-service/migrations/`): `0001_init`,
`0002_task_project_execution_tracking`, `0003_task_workflow_template_id`
exist today — `0004` is the genuine next-free number, matching both
BE-SOL-001's and BE-SOL-003's own citation.

**This migration is shared with BE-SOL-003** — BE-SOL-003's own affected-files
list says explicitly: *"fold `grants.expires_at`, `tasks.share_token` into
the same migration as BE-SOL-001, since both touch `task.tasks`/
`task.grants` — avoid two migrations touching adjacent tables back-to-back"*
(BE-SOL-003 §Affected files). This task (TASK-TG-001-01) owns writing
`0004_task_fields_and_comments.{up,down}.sql` with BE-SOL-001's columns only.
**TASK-TG-003-03** (expiry) and **TASK-TG-003-05** (share-link) each append
their own `ALTER TABLE` statements to this SAME file rather than creating
`0005`/`0006` — read this task's Context before touching either of those,
and if this task hasn't landed yet when TASK-TG-003-03/-05 start, coordinate
on one shared PR instead of three competing edits to the same file.

**Correctness catch versus BE-SOL-001's own sketch**: its "Design — schema
widening" section includes the line
`CREATE TABLE task.task_comments_index();` labeled "placeholder note:
task.task_comments already exists" — this is prose masquerading as SQL, not
something to actually run. **Do not include this line in the real
migration.** `task.task_comments` already exists with RLS
(`backend-go/services/task-service/migrations/0001_init.up.sql:82-93`,
confirmed by direct read) and needs zero migration changes — this task only
adds the `task.tasks` columns and the widened status CHECK constraint.

The current `task.tasks.status` CHECK constraint is an unnamed inline
constraint on a 4-value set (verified,
`migrations/0001_init.up.sql:16`: `status TEXT NOT NULL DEFAULT 'open' CHECK
(status IN ('open', 'in_progress', 'done', 'cancelled'))`). Postgres
auto-names an inline single-column CHECK `<table>_<column>_check`, so
`tasks_status_check` (the name BE-SOL-001's migration drops) is correct as
written — confirmed by Postgres's default naming convention, not something
this task needs to re-derive by inspecting `pg_constraint` at migration-write
time, but worth a `\d task.tasks` sanity check against a real dev DB before
merging if in doubt.

## Changes to make

`backend-go/services/task-service/migrations/0004_task_fields_and_comments.up.sql`:

```sql
ALTER TABLE task.tasks
  ADD COLUMN description      TEXT,
  ADD COLUMN type              TEXT,
  ADD COLUMN priority          TEXT,
  ADD COLUMN labels            TEXT[] DEFAULT '{}',
  ADD COLUMN assignee_id       TEXT,
  ADD COLUMN reporter_id       TEXT,
  ADD COLUMN owner_id          TEXT,
  ADD COLUMN due_date          TIMESTAMPTZ,
  ADD COLUMN estimated_hours   NUMERIC,
  ADD COLUMN actual_hours      NUMERIC,
  ADD COLUMN prompt_template   TEXT,
  ADD COLUMN ai_context        JSONB,
  ADD COLUMN ai_plan_json      JSONB,
  ADD COLUMN visibility        TEXT NOT NULL DEFAULT 'private',
  ADD COLUMN worktree_id       TEXT,
  ADD COLUMN agent_session_id  TEXT,
  ADD COLUMN workflow_exec_id  TEXT,
  ADD COLUMN done_subtasks     INT NOT NULL DEFAULT 0,
  ADD COLUMN total_subtasks    INT NOT NULL DEFAULT 0;

ALTER TABLE task.tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('backlog','todo','open','in_progress','blocked','review','done','cancelled'));
-- 'open' is kept alongside 'backlog'/'todo' for backward compatibility with
-- existing rows written under the current 4-value enum — no data backfill
-- required for this migration to stay valid.
```

Do NOT add the `task.task_comments_index` line — see Context above.

`backend-go/services/task-service/migrations/0004_task_fields_and_comments.down.sql`:

```sql
ALTER TABLE task.tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('open', 'in_progress', 'done', 'cancelled'));

ALTER TABLE task.tasks
  DROP COLUMN IF EXISTS total_subtasks,
  DROP COLUMN IF EXISTS done_subtasks,
  DROP COLUMN IF EXISTS workflow_exec_id,
  DROP COLUMN IF EXISTS agent_session_id,
  DROP COLUMN IF EXISTS worktree_id,
  DROP COLUMN IF EXISTS visibility,
  DROP COLUMN IF EXISTS ai_plan_json,
  DROP COLUMN IF EXISTS ai_context,
  DROP COLUMN IF EXISTS prompt_template,
  DROP COLUMN IF EXISTS actual_hours,
  DROP COLUMN IF EXISTS estimated_hours,
  DROP COLUMN IF EXISTS due_date,
  DROP COLUMN IF EXISTS owner_id,
  DROP COLUMN IF EXISTS reporter_id,
  DROP COLUMN IF EXISTS assignee_id,
  DROP COLUMN IF EXISTS labels,
  DROP COLUMN IF EXISTS priority,
  DROP COLUMN IF EXISTS type,
  DROP COLUMN IF EXISTS description;
```

`owner_id` is deliberately left NULL on this migration (no backfill) — see
BE-SOL-001's own note: a NULL `owner_id` is "no intrinsic owner" for
TASK-TG-003-02's grant logic to fall through on; backfilling
`owner_id = created_by` for pre-existing rows is a production rollout
decision, not part of this task.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v

# Manual up/down round-trip if testcontainers/Docker isn't available:
migrate -path services/task-service/migrations -database "$DATABASE_DSN_TASK" up
migrate -path services/task-service/migrations -database "$DATABASE_DSN_TASK" down 1
```

Expected: migration applies cleanly on top of `0001`-`0003`; `down` cleanly
reverses it (constraint restored to the original 4-value set before the new
columns are dropped — order matters since the down migration's constraint
recreation must run first, matching the up migration's own drop-then-add
ordering in reverse); no existing `task.tasks` row is affected (every new
column is nullable or has a safe default, no NOT NULL without DEFAULT).
