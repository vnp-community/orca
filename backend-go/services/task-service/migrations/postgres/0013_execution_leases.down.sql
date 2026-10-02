DROP INDEX IF EXISTS task.idx_execution_links_expired_lease;
ALTER TABLE task.execution_links
  DROP COLUMN IF EXISTS previous_status,
  DROP COLUMN IF EXISTS lease_owner,
  DROP COLUMN IF EXISTS lease_expires_at;
