-- Widens task.tasks with the fields BE-SOL-001's data model needs
-- (description/type/priority/labels/assignment/ownership/estimates/AI
-- context/visibility/execution-tracking/subtask counters), broadens the
-- status CHECK to the new state machine, and (folded into this same
-- migration per TASK-TG-001-01's Context note, to avoid two migrations
-- touching adjacent task.tasks/task.task_grants tables back-to-back)
-- appends BE-SOL-003's task.task_grants.expires_at and task.tasks.share_token
-- columns for grant expiry and public share-link support.
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

-- owner_id is deliberately left NULL here (no backfill) — a NULL owner_id
-- means "no intrinsic owner" for TASK-TG-003-02's grant logic to fall
-- through on; backfilling owner_id = created_by is a production rollout
-- decision, not part of this migration.

-- TASK-TG-003-03: grant expiry, filtered at read time by ResolvePermission.
ALTER TABLE task.task_grants ADD COLUMN expires_at TIMESTAMPTZ;

-- TASK-TG-003-05: public share-link lookup key. UNIQUE is load-bearing —
-- two tasks sharing a token would let GetTaskByShareToken leak the wrong
-- task.
ALTER TABLE task.tasks ADD COLUMN share_token TEXT UNIQUE;
CREATE INDEX idx_tasks_share_token ON task.tasks (share_token) WHERE share_token IS NOT NULL;
