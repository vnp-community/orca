ALTER TABLE task.tasks
  DROP COLUMN IF EXISTS active_execution_link_id;

DROP TABLE IF EXISTS task.execution_links;
