-- MySQL/TiDB variant of postgres/0005_execution_ad_hoc_template.up.sql —
-- ExecuteAdHocStep's synthetic, templateless execution needs template_id
-- to accept NULL. MySQL requires dropping and re-adding the foreign key to
-- change its ON DELETE action (no single ALTER for both at once).
ALTER TABLE executions MODIFY COLUMN template_id CHAR(36) NULL;
ALTER TABLE executions DROP FOREIGN KEY fk_workflow_executions_template;
ALTER TABLE executions ADD CONSTRAINT fk_workflow_executions_template
    FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE SET NULL;
