DROP INDEX idx_tasks_project_task_number ON tasks;
DROP TABLE IF EXISTS task_number_seq;

ALTER TABLE tasks DROP COLUMN pr_url;
ALTER TABLE tasks DROP COLUMN task_number;
