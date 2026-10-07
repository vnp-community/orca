DROP INDEX uq_tasks_active_plan_per_request ON tasks;
DROP INDEX idx_tasks_request ON tasks;
ALTER TABLE tasks DROP COLUMN active_plan_request_id;
ALTER TABLE tasks DROP COLUMN request_id;
