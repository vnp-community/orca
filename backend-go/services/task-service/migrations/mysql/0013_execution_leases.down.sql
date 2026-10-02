DROP INDEX idx_execution_links_expired_lease ON execution_links;
ALTER TABLE execution_links
  DROP COLUMN previous_status,
  DROP COLUMN lease_owner,
  DROP COLUMN lease_expires_at;
