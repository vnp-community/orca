DROP INDEX IF EXISTS idx_infra_ssh_targets_tenant_host_user;
ALTER TABLE infra.ssh_targets
  DROP COLUMN IF EXISTS tags,
  DROP COLUMN IF EXISTS project;
