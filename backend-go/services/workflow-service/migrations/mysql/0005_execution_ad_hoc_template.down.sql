ALTER TABLE executions DROP FOREIGN KEY fk_workflow_executions_template;
ALTER TABLE executions ADD CONSTRAINT fk_workflow_executions_template
    FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE;
ALTER TABLE executions MODIFY COLUMN template_id CHAR(36) NOT NULL;
