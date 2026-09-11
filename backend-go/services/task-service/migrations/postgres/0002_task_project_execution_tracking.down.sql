DROP INDEX IF EXISTS task.idx_tasks_project_active;
ALTER TABLE task.tasks DROP COLUMN IF EXISTS project_id;
