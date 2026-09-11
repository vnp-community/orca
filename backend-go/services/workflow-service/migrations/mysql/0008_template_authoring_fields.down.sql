DROP INDEX idx_workflow_templates_owner ON templates;
ALTER TABLE templates DROP FOREIGN KEY fk_workflow_templates_cloned_from;
ALTER TABLE templates
  DROP COLUMN cloned_from_template_id,
  DROP COLUMN remove_steps,
  DROP COLUMN inject_steps,
  DROP COLUMN overrides,
  DROP COLUMN usage_count,
  DROP COLUMN owner_id,
  DROP COLUMN tags,
  DROP COLUMN description;
