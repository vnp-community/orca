DROP INDEX idx_repos_dev_server ON repos;
ALTER TABLE repos DROP COLUMN dev_server_id;
