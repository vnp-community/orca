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
