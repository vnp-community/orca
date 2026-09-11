-- MySQL translation of migrations/postgres/0003_task_fields_and_comments.up.sql.
-- NUMERIC(6,2) -> DECIMAL(6,2) (MySQL's canonical spelling; NUMERIC is
-- accepted as a synonym but DECIMAL is what MySQL actually stores it as).
-- ai_plan_json JSONB -> JSON (dbcapability.Capabilities.SupportsJSONB is
-- false for MySQL — loses the ->/@> operators, but this column is only
-- ever written/read whole, via UpdateAIPlanJSON/scanTask, never queried by
-- a JSONB operator, so nothing behavioral is lost).
ALTER TABLE tasks
  ADD COLUMN description        TEXT,
  ADD COLUMN task_type          VARCHAR(10) NOT NULL DEFAULT 'task',
  ADD COLUMN priority            VARCHAR(10) NOT NULL DEFAULT 'medium',
  ADD COLUMN assignee_id         CHAR(36) NULL,          -- logical FK -> tenant-service
  ADD COLUMN owner_id            CHAR(36) NULL,          -- logical FK -> tenant-service; see SOL-TG-03
  ADD COLUMN due_date            TIMESTAMP(6) NULL,
  ADD COLUMN estimated_hours     DECIMAL(6,2),
  ADD COLUMN actual_hours        DECIMAL(6,2),           -- see SOL-TG-04
  ADD COLUMN prompt_template     TEXT,                    -- see SOL-TG-02
  ADD COLUMN ai_context          TEXT,
  ADD COLUMN ai_plan_json        JSON,                    -- see SOL-TG-02
  ADD COLUMN visibility          VARCHAR(10) NOT NULL DEFAULT 'team',
  ADD COLUMN worktree_id         CHAR(36) NULL,           -- logical FK -> project-service worktrees
  ADD COLUMN agent_session_id    TEXT,                    -- see SOL-TG-04
  ADD COLUMN progress_percent    SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE tasks
  ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN ('task','bug','feature','epic')),
  ADD CONSTRAINT tasks_priority_check CHECK (priority IN ('low','medium','high','urgent')),
  ADD CONSTRAINT tasks_visibility_check CHECK (visibility IN ('private','team','public')),
  ADD CONSTRAINT tasks_progress_percent_check CHECK (progress_percent BETWEEN 0 AND 100);

-- StatusBlocked/StatusReview join the status CHECK — named constraint from
-- migrations/mysql/0001 lets this DROP/ADD target the exact same
-- constraint migrations/postgres/0003 replaces (see that migration's
-- equivalent DROP CONSTRAINT tasks_status_check).
ALTER TABLE tasks DROP CHECK tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('open','blocked','in_progress','review','done','cancelled'));

-- Postgres's idx_tasks_assignee is a PARTIAL index (WHERE assignee_id IS
-- NOT NULL) — no MySQL equivalent; full index instead, same reasoning as
-- migrations/mysql/0002's idx_tasks_project_active.
CREATE INDEX idx_tasks_assignee ON tasks (assignee_id);
