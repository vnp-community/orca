DROP TABLE IF EXISTS task_share_links;

DROP INDEX idx_task_grants_expires ON task_grants;
ALTER TABLE task_grants DROP COLUMN expires_at;
