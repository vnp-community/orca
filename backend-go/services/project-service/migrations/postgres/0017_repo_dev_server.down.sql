DROP INDEX IF EXISTS project.idx_repos_dev_server;
ALTER TABLE project.repos DROP COLUMN IF EXISTS dev_server_id;
