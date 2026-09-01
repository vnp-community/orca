-- CR-DS-006 (docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md)
-- Phase 1 — data model only. approval_status is NOT enforced anywhere yet
-- (no RPC/policy reads it to gate access) — see that CR's §3
-- "Chưa làm ở Phase 1".
--
-- Column named approval_status, NOT status: infra.dev_servers already has a
-- `status` column (added by a separate, concurrently-in-flight migration —
-- health/bootstrap status: pending|healthy|degraded|unhealthy). This is a
-- different concept — admin approval workflow state — so it gets its own
-- column rather than overloading or fighting over that one.

CREATE TABLE infra.dev_server_groups (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    name                TEXT NOT NULL,
    parent_group_id     UUID REFERENCES infra.dev_server_groups(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_dev_server_groups_tenant ON infra.dev_server_groups (tenant_id);

-- Same RLS-as-defense-in-depth pattern as every other table in this
-- schema (0001_init.up.sql) — the repository layer's explicit tenant_id
-- filtering is primary enforcement, this is the backstop.
ALTER TABLE infra.dev_server_groups ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.dev_server_groups
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

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
ALTER TABLE infra.dev_servers
    ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'approved'
        CHECK (approval_status IN ('pending_approval', 'approved', 'rejected')),
    ADD COLUMN group_id UUID REFERENCES infra.dev_server_groups(id);
