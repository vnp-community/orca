DROP INDEX IF EXISTS task.idx_tasks_share_token;
ALTER TABLE task.tasks DROP COLUMN IF EXISTS share_token;

ALTER TABLE task.tasks
  DROP COLUMN IF EXISTS total_subtasks,
  DROP COLUMN IF EXISTS done_subtasks,
  DROP COLUMN IF EXISTS workflow_exec_id,
  DROP COLUMN IF EXISTS reporter_id,
  DROP COLUMN IF EXISTS labels;
