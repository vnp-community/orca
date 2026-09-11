-- Links infra.dev_servers to infra.ssh_targets — required for relay-ssh
-- mode's cert-based SSH auth (internal/adapter/sshconn.Connector dials the
-- referenced SshTarget), empty/NULL for the other two connection modes. See
-- domain.NewDevServer's ErrMissingSSHTargetForRelaySSH invariant and
-- specs/backend-go/services/infra-fleet-service.md §9.
ALTER TABLE infra.dev_servers
    ADD COLUMN ssh_target_id UUID REFERENCES infra.ssh_targets(id);

CREATE INDEX idx_infra_dev_servers_ssh_target ON infra.dev_servers (ssh_target_id)
    WHERE ssh_target_id IS NOT NULL;
