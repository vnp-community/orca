-- MySQL does not support "DROP COLUMN IF EXISTS" — see migrations/mysql/
-- 0014_connections_grace_period.down.sql's comment.
ALTER TABLE dev_servers
  DROP COLUMN last_provisioned_at,
  DROP COLUMN agent_version,
  DROP COLUMN node_version,
  DROP COLUMN arch,
  DROP COLUMN platform,
  DROP COLUMN status;
