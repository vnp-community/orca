CREATE TABLE IF NOT EXISTS issuetracking.connections (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             TEXT NOT NULL,
    user_id               TEXT NOT NULL,
    provider              TEXT NOT NULL,
    external_workspace_id TEXT NOT NULL,
    workspace_name        TEXT NOT NULL DEFAULT '',
    workspace_url         TEXT NOT NULL DEFAULT '',
    viewer_id             TEXT NOT NULL DEFAULT '',
    viewer_display_name   TEXT NOT NULL DEFAULT '',
    viewer_email          TEXT NOT NULL DEFAULT '',
    credential_id         UUID NOT NULL,
    is_selected           BOOLEAN NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issuetracking_connections_site_key
        UNIQUE (tenant_id, user_id, provider, external_workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_issuetracking_connections_lookup
    ON issuetracking.connections (tenant_id, user_id, provider);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering
-- (internal/adapter/postgres/connections.go) is the primary enforcement;
-- this is the secondary backstop, matching outbox_events' own policy
-- (0001_outbox.up.sql). tenant_id here is TEXT, not UUID (see
-- ports.go/tenant.RequireTenantID's string contract), so no ::uuid cast.
ALTER TABLE issuetracking.connections ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON issuetracking.connections
    USING (tenant_id = current_setting('app.tenant_id', true));
