# TASK-TG-01-02: Migration `0003` — widen `task.tasks`, activate `task_comments`

**From Solution:** SOL-TG-01
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/migrations/0003_task_fields_and_comments.up.sql` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`task.task_comments` already exists (`migrations/0001_init.up.sql:82-93`)
but is unused — no repository/usecase/RPC reads or writes it. `task.tasks`
is missing every field beyond the scaffold's original 6. This migration is
purely additive (new nullable/defaulted columns, one widened CHECK
constraint) — no data migration needed since every new column is optional
or has a safe default.

## Changes to make

Create `backend-go/services/task-service/migrations/0003_task_fields_and_comments.up.sql`:

```sql
ALTER TABLE task.tasks
  ADD COLUMN description        TEXT,
  ADD COLUMN task_type          TEXT NOT NULL DEFAULT 'task' CHECK (task_type IN ('task','bug','feature','epic')),
  ADD COLUMN priority            TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low','medium','high','urgent')),
  ADD COLUMN assignee_id         UUID,          -- logical FK -> tenant-service
  ADD COLUMN owner_id            UUID,          -- logical FK -> tenant-service; see SOL-TG-03's owner-intrinsic short-circuit
  ADD COLUMN due_date            TIMESTAMPTZ,
  ADD COLUMN estimated_hours     NUMERIC(6,2),
  ADD COLUMN actual_hours        NUMERIC(6,2),  -- see SOL-TG-04 (auto-advance to review)
  ADD COLUMN prompt_template     TEXT,          -- see SOL-TG-02 ("Generate Agent Prompt")
  ADD COLUMN ai_context          TEXT,
  ADD COLUMN ai_plan_json        JSONB,         -- see SOL-TG-02 (raw AI response)
  ADD COLUMN visibility          TEXT NOT NULL DEFAULT 'team' CHECK (visibility IN ('private','team','public')),
  ADD COLUMN worktree_id         UUID,          -- logical FK -> project-service worktrees; see SOL-TG-04
  ADD COLUMN agent_session_id    TEXT,          -- see SOL-TG-04
  ADD COLUMN progress_percent    SMALLINT NOT NULL DEFAULT 0 CHECK (progress_percent BETWEEN 0 AND 100);

-- StatusBlocked/StatusReview join the status CHECK — see domain/task.go's
-- new StatusBlocked/StatusReview consts and TASK-TG-01-07's auto-block design.
ALTER TABLE task.tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('open','blocked','in_progress','review','done','cancelled'));

CREATE INDEX idx_tasks_assignee ON task.tasks (assignee_id) WHERE assignee_id IS NOT NULL;
```

Create `backend-go/services/task-service/migrations/0003_task_fields_and_comments.down.sql`:

```sql
DROP INDEX IF EXISTS task.idx_tasks_assignee;

ALTER TABLE task.tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('open', 'in_progress', 'done', 'cancelled'));

ALTER TABLE task.tasks
  DROP COLUMN description,
  DROP COLUMN task_type,
  DROP COLUMN priority,
  DROP COLUMN assignee_id,
  DROP COLUMN owner_id,
  DROP COLUMN due_date,
  DROP COLUMN estimated_hours,
  DROP COLUMN actual_hours,
  DROP COLUMN prompt_template,
  DROP COLUMN ai_context,
  DROP COLUMN ai_plan_json,
  DROP COLUMN visibility,
  DROP COLUMN worktree_id,
  DROP COLUMN agent_session_id,
  DROP COLUMN progress_percent;
```

No DDL is needed for `task.task_comments` — its existing columns
(`id, tenant_id, task_id, author_id, content, created_at`) already match
what `AddComment`/`ListComments` need (see `TASK-TG-01-05`).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/task-service
# Requires a local Postgres reachable via DATABASE_DSN / migrate tool
# already used by this service's other migrations — confirm the up/down/up
# cycle is clean, per 05-data-architecture.md's migration-testing convention:
migrate -database "$DATABASE_DSN" -path migrations up
migrate -database "$DATABASE_DSN" -path migrations down 1
migrate -database "$DATABASE_DSN" -path migrations up
```

Expected: all three steps succeed with no errors; `\d task.tasks` in `psql`
shows every new column and the widened `tasks_status_check` constraint after
the final `up`.
