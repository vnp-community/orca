-- MySQL translation of migrations/postgres/0009_task_workflow_template_id.up.sql.
-- No FK — workflow.templates lives in workflow-service's own database
-- (database-per-service), same cross-service reference convention
-- project_id already uses.
ALTER TABLE tasks ADD COLUMN workflow_template_id CHAR(36) NULL;
