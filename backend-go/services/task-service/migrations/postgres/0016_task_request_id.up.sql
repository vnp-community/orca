ALTER TABLE task.tasks ADD COLUMN request_id UUID;
CREATE INDEX idx_tasks_request ON task.tasks (tenant_id, request_id) WHERE request_id IS NOT NULL;
CREATE UNIQUE INDEX uq_tasks_active_plan_per_request ON task.tasks (tenant_id, request_id)
  WHERE task_type = 'plan' AND status <> 'cancelled';
