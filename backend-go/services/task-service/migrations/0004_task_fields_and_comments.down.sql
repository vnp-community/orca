DROP INDEX IF EXISTS task.idx_tasks_share_token;
ALTER TABLE task.tasks DROP COLUMN IF EXISTS share_token;

ALTER TABLE task.task_grants DROP COLUMN IF EXISTS expires_at;

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
