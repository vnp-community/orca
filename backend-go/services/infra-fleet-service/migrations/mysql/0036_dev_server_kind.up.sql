-- CR-DS-009 (docs/crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md)
-- §3.1 — AgentKind distinguishes a Dev Server Agent registration (agent/)
-- from a Mobile Emulator Agent registration (emulator/); both register
-- through the same RegisterDevServer RPC and share this same registry.
--
-- Default 'dev_server' backfills every existing row (Postgres applies the
-- default to pre-existing rows for a NOT NULL ADD COLUMN, same pattern as
-- migrations/0008's approval_status column) — every dev server registered
-- before this migration is a Dev Server Agent, since AGENT_KIND_MOBILE_EMULATOR
-- did not exist yet.
-- VARCHAR, not TEXT, for this CHECK-constrained enum column — see
-- migrations/mysql/0013_ephemeral_vm_runtimes.up.sql's comment.
ALTER TABLE dev_servers
    ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT 'dev_server'
        CHECK (kind IN ('dev_server', 'mobile_emulator'));
