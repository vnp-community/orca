ALTER TABLE infra.dev_servers
  DROP COLUMN IF EXISTS last_provisioned_at,
  DROP COLUMN IF EXISTS agent_version,
  DROP COLUMN IF EXISTS node_version,
  DROP COLUMN IF EXISTS arch,
  DROP COLUMN IF EXISTS platform,
  DROP COLUMN IF EXISTS status;
