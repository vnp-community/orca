-- Fills in port/known-hosts/jump-host from the fuller design-doc entity
-- (specs/backend-go/services/infra-fleet-service.md §5) that
-- domain.SshTarget's original scaffold left unmodeled. See SOL-SSH-01.
ALTER TABLE ssh_targets ADD COLUMN port INTEGER NOT NULL DEFAULT 22;
-- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
-- expression — see migrations/mysql/0002_connections.up.sql's comment.
ALTER TABLE ssh_targets ADD COLUMN known_hosts_fingerprint TEXT NOT NULL DEFAULT ('');
ALTER TABLE ssh_targets ADD COLUMN jump_host_target_id CHAR(36) REFERENCES ssh_targets(id);
