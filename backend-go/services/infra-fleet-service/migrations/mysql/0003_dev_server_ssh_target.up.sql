-- Links dev_servers to ssh_targets — required for relay-ssh
-- mode's cert-based SSH auth (internal/adapter/sshconn.Connector dials the
-- referenced SshTarget), empty/NULL for the other two connection modes. See
-- domain.NewDevServer's ErrMissingSSHTargetForRelaySSH invariant and
-- specs/backend-go/services/infra-fleet-service.md §9.
ALTER TABLE dev_servers
    ADD COLUMN ssh_target_id CHAR(36) REFERENCES ssh_targets(id);

-- MySQL/TiDB has no partial index — full index over the (nullable) column
-- instead of Postgres's `WHERE ssh_target_id IS NOT NULL`; only a lookup
-- cost difference (a few extra NULL entries indexed), not a correctness one.
CREATE INDEX idx_infra_dev_servers_ssh_target ON dev_servers (ssh_target_id);
