-- MySQL/TiDB variant of postgres/0002_execution_project_id.up.sql.
ALTER TABLE executions ADD COLUMN project_id CHAR(36) NULL;

-- MySQL has no partial index (Postgres's `WHERE status IN (...)`) — indexes
-- the full (tenant_id, project_id, status) triple instead; still correct,
-- just not restricted to only the active-status rows.
CREATE INDEX idx_workflow_executions_project_active ON executions (tenant_id, project_id, status);
