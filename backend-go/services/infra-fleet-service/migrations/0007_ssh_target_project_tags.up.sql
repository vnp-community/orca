ALTER TABLE infra.ssh_targets
  ADD COLUMN project TEXT NOT NULL DEFAULT '',
  ADD COLUMN tags     TEXT[] NOT NULL DEFAULT '{}';

-- Upsert-by-hostname+user (BL-FLEET-01's "INSERT OR UPDATE by hostname+user")
-- needs this uniqueness constraint to exist at all — it does not today.
CREATE UNIQUE INDEX idx_infra_ssh_targets_tenant_host_user
  ON infra.ssh_targets (tenant_id, host, user_name);
