DROP INDEX idx_tasks_project_active ON tasks;
ALTER TABLE tasks DROP COLUMN project_id;
