DROP INDEX IF EXISTS task.uq_tasks_active_plan_per_request;
DROP INDEX IF EXISTS task.idx_tasks_request;
ALTER TABLE task.tasks DROP COLUMN request_id;
