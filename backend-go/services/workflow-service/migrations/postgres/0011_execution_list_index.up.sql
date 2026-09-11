-- Supports ListExecutions' (TASK-WF-005-01) keyset pagination —
-- tenant_id + optional project_id filter, ordered by created_at DESC.
-- Neither idx_workflow_executions_tenant_status (0001_init, status-first)
-- nor idx_workflow_executions_project_active (0002, partial on active
-- statuses only) covers an unfiltered-by-status, newest-first scan.
CREATE INDEX idx_workflow_executions_tenant_project_created ON workflow.executions (tenant_id, project_id, created_at DESC, id DESC);
