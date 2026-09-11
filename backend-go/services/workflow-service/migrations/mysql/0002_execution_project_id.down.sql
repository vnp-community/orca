DROP INDEX idx_workflow_executions_project_active ON executions;
ALTER TABLE executions DROP COLUMN project_id;
