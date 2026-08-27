DROP INDEX IF EXISTS infra.idx_infra_dev_servers_ssh_target;
ALTER TABLE infra.dev_servers DROP COLUMN IF EXISTS ssh_target_id;
