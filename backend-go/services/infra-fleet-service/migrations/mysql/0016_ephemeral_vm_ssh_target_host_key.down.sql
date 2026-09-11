-- MySQL does not support "DROP COLUMN IF EXISTS" — see migrations/mysql/
-- 0014_connections_grace_period.down.sql's comment.
ALTER TABLE ephemeral_vm_ssh_targets
  DROP COLUMN host_key_fingerprint;
