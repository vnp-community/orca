-- CR-DS-006 (docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md)
-- Phase 1 — data model only. approval_status is NOT enforced anywhere yet
-- (no RPC/policy reads it to gate access) — see that CR's §3
-- "Chưa làm ở Phase 1".
--
-- Column named approval_status, NOT status: dev_servers already has a
-- `status` column (added by a separate, concurrently-in-flight migration —
-- health/bootstrap status: pending|healthy|degraded|unhealthy). This is a
-- different concept — admin approval workflow state — so it gets its own
-- column rather than overloading or fighting over that one.

CREATE TABLE dev_server_groups (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    name                TEXT NOT NULL,
    parent_group_id     CHAR(36) REFERENCES dev_server_groups(id),
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_infra_dev_server_groups_tenant ON dev_server_groups (tenant_id);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.

-- Column default 'approved' (not 'pending_approval') protects any row
-- inserted without going through the Go domain layer (e.g. a future direct
-- SQL script) from silently landing in a status-gated state once Phase 2
-- starts enforcing it. The Go-level default the application actually uses
-- for freshly-registered dev servers is 'pending_approval', set explicitly
-- by domain.NewDevServer — see that function's doc comment. ADD COLUMN ...
-- DEFAULT backfills every existing row with 'approved' as part of this same
-- ALTER (Postgres applies the default to pre-existing rows for a NOT NULL
-- ADD COLUMN) — a dev server registered before this migration must not
-- retroactively become locked out.
-- VARCHAR, not TEXT, for this CHECK-constrained enum column — see
-- migrations/mysql/0013_ephemeral_vm_runtimes.up.sql's comment.
ALTER TABLE dev_servers
    ADD COLUMN approval_status VARCHAR(20) NOT NULL DEFAULT 'approved'
        CHECK (approval_status IN ('pending_approval', 'approved', 'rejected')),
    ADD COLUMN group_id CHAR(36) REFERENCES dev_server_groups(id);
