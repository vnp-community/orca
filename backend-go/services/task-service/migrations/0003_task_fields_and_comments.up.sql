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
