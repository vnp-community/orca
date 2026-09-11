DROP INDEX idx_tasks_assignee ON tasks;

ALTER TABLE tasks DROP CHECK tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('open', 'in_progress', 'done', 'cancelled'));

ALTER TABLE tasks
  DROP CHECK tasks_task_type_check,
  DROP CHECK tasks_priority_check,
  DROP CHECK tasks_visibility_check,
  DROP CHECK tasks_progress_percent_check;

ALTER TABLE tasks
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
