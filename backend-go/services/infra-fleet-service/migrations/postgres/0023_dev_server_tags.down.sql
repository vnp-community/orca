DROP INDEX IF EXISTS infra.idx_infra_dev_servers_tags;
ALTER TABLE infra.dev_servers DROP COLUMN tags;
