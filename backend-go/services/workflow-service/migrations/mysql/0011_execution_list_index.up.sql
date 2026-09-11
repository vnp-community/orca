-- Supports ListExecutions' keyset pagination — same composite index as the
-- Postgres variant; MySQL 8.0+ supports true DESC indexes (see
-- 0009_template_visibility_sharing.up.sql's comment on
-- idx_workflow_templates_trending for the same point).
CREATE INDEX idx_workflow_executions_tenant_project_created ON executions (tenant_id, project_id, created_at DESC, id DESC);
