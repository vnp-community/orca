-- MySQL translation of migrations/postgres/0002_task_project_execution_tracking.up.sql.
-- Same HONEST CAVEAT as the Postgres version: status = 'in_progress' is a
-- one-way transition in this scaffold — see that migration's comment.
ALTER TABLE tasks ADD COLUMN project_id CHAR(36) NULL;

-- Postgres's idx_tasks_project_active is a PARTIAL index
-- (WHERE status = 'in_progress') — MySQL has no partial index, so this is a
-- full composite index instead. Functionally identical for every query this
-- service runs (HasActiveExecutions's EXISTS(... WHERE status =
-- 'in_progress') still uses it); only larger on disk than Postgres's
-- narrower partial index, no query-result difference.
CREATE INDEX idx_tasks_project_active ON tasks (tenant_id, project_id, status);
