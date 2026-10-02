-- MySQL translation of migrations/postgres/0013_execution_leases.up.sql.
-- MySQL has no partial index, so the sweep index covers the filter columns.
ALTER TABLE execution_links
  ADD COLUMN lease_expires_at TIMESTAMP(6) NULL,
  ADD COLUMN lease_owner      VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN previous_status  VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX idx_execution_links_expired_lease
  ON execution_links (engine, status_mirror, lease_expires_at);
