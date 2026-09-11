-- Fills in port/known-hosts/jump-host from the fuller design-doc entity
-- (specs/backend-go/services/infra-fleet-service.md §5) that
-- domain.SshTarget's original scaffold left unmodeled. See SOL-SSH-01.
ALTER TABLE infra.ssh_targets ADD COLUMN port INTEGER NOT NULL DEFAULT 22;
ALTER TABLE infra.ssh_targets ADD COLUMN known_hosts_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE infra.ssh_targets ADD COLUMN jump_host_target_id UUID REFERENCES infra.ssh_targets(id);
