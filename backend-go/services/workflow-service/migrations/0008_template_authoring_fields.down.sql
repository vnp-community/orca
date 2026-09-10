DROP INDEX IF EXISTS workflow.idx_workflow_templates_owner;
DROP INDEX IF EXISTS workflow.idx_workflow_templates_tags;
ALTER TABLE workflow.templates
  DROP COLUMN cloned_from_template_id,
  DROP COLUMN remove_steps,
  DROP COLUMN inject_steps,
  DROP COLUMN overrides,
  DROP COLUMN usage_count,
  DROP COLUMN owner_id,
  DROP COLUMN tags,
  DROP COLUMN description;
