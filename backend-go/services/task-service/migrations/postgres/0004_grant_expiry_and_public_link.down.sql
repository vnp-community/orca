DROP TABLE IF EXISTS task.task_share_links;

DROP INDEX IF EXISTS task.idx_task_grants_expires;
ALTER TABLE task.task_grants DROP COLUMN expires_at;
