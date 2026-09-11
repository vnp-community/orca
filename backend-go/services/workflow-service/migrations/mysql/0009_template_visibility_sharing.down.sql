DROP TABLE approvals;
DROP TABLE ratings;
DROP INDEX idx_workflow_templates_fts ON templates;
DROP INDEX idx_workflow_templates_trending ON templates;
DROP INDEX idx_workflow_templates_visibility ON templates;
ALTER TABLE templates
  DROP COLUMN rating_count, DROP COLUMN rating_sum, DROP COLUMN share_token, DROP COLUMN visibility;
