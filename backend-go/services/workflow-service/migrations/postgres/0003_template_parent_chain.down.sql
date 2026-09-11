DROP INDEX IF EXISTS workflow.idx_workflow_templates_parent;
ALTER TABLE workflow.templates DROP COLUMN IF EXISTS parent_template_id;
