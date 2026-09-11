ALTER TABLE workflow.executions DROP CONSTRAINT executions_template_id_fkey;
ALTER TABLE workflow.executions ADD CONSTRAINT executions_template_id_fkey
    FOREIGN KEY (template_id) REFERENCES workflow.templates(id) ON DELETE CASCADE;
ALTER TABLE workflow.executions ALTER COLUMN template_id SET NOT NULL;
