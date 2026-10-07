ALTER TABLE tasks ADD COLUMN request_id CHAR(36) NULL;
ALTER TABLE tasks ADD COLUMN active_plan_request_id CHAR(36) GENERATED ALWAYS AS (CASE WHEN task_type = 'plan' AND status <> 'cancelled' THEN request_id END) STORED;
CREATE INDEX idx_tasks_request ON tasks (tenant_id, request_id);
CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON tasks (tenant_id, active_plan_request_id);
