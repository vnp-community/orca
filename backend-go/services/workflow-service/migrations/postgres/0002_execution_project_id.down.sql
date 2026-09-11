DROP INDEX IF EXISTS idx_workflow_executions_project_active;
ALTER TABLE workflow.executions DROP COLUMN IF EXISTS project_id;
