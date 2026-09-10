DROP INDEX IF EXISTS task.idx_tasks_project_task_number;
DROP SEQUENCE IF EXISTS task.task_number_seq;

ALTER TABLE task.tasks DROP COLUMN IF EXISTS pr_url;
ALTER TABLE task.tasks DROP COLUMN IF EXISTS task_number;
