DROP TABLE workflow.approvals;
DROP TABLE workflow.ratings;
DROP INDEX IF EXISTS workflow.idx_workflow_templates_fts;
DROP INDEX IF EXISTS workflow.idx_workflow_templates_trending;
DROP INDEX IF EXISTS workflow.idx_workflow_templates_share_token;
DROP INDEX IF EXISTS workflow.idx_workflow_templates_visibility;
ALTER TABLE workflow.templates
  DROP COLUMN rating_count, DROP COLUMN rating_sum, DROP COLUMN share_token, DROP COLUMN visibility;
