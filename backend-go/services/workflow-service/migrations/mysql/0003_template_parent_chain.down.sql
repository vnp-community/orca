-- FK must be dropped BEFORE the index it depends on (confirmed empirically:
-- MySQL/InnoDB refuses DROP INDEX with error 1553 "needed in a foreign key
-- constraint" if the owning FK is still present) — reverse of the up
-- migration's natural ADD order.
ALTER TABLE templates DROP FOREIGN KEY fk_workflow_templates_parent;
DROP INDEX idx_workflow_templates_parent ON templates;
ALTER TABLE templates DROP COLUMN parent_template_id;
