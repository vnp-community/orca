-- OAuth consent requests and grants (BE-MCP-SOL-005 section D). The OAuth
-- client registry and tokens live in auth-service; user/client ids here are
-- logical references only (no cross-database FKs).
CREATE TABLE mcp.consent_requests (
    id                 UUID PRIMARY KEY,
    tenant_id          UUID NOT NULL,
    user_id            UUID NOT NULL,
    client_id          TEXT NOT NULL,
    client_name        TEXT NOT NULL,
    client_uri         TEXT,
    redirect_uri       TEXT NOT NULL,
    scopes             TEXT[] NOT NULL,
    state              TEXT CHECK (state IS NULL OR char_length(state) <= 512),
    code_challenge     TEXT NOT NULL,
    resource           TEXT NOT NULL,
    is_new_client      BOOLEAN NOT NULL,
    registered_via_dcr BOOLEAN NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,
    decided_at         TIMESTAMPTZ,
    decision           TEXT CHECK (decision IN ('approve', 'deny'))
);
CREATE INDEX idx_mcp_consent_expiry ON mcp.consent_requests (expires_at);

CREATE TABLE mcp.grants (
    id                       UUID PRIMARY KEY,
    tenant_id                UUID NOT NULL,
    user_id                  UUID NOT NULL,
    client_id                TEXT NOT NULL,
    client_name              TEXT NOT NULL,
    client_uri               TEXT,
    scopes                   TEXT[] NOT NULL,
    status                   TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at             TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ,
    revoked_by               UUID,
    -- NULL while status = 'revoked' means auth-service still has to be told.
    revocation_propagated_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_grants_active ON mcp.grants (tenant_id, user_id, client_id) WHERE status = 'active';
CREATE INDEX idx_mcp_grants_tenant ON mcp.grants (tenant_id, status);
CREATE INDEX idx_mcp_grants_unpropagated ON mcp.grants (revoked_at)
    WHERE status = 'revoked' AND revocation_propagated_at IS NULL;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['consent_requests', 'grants'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
        WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;

-- The revocation reconciler scans pending revocations across tenants. Like the
-- outbox relay it opts in per transaction (app.relay), is read-only, and sees
-- only rows still waiting for propagation. Marking a grant propagated is done
-- in an ordinary tenant-scoped transaction.
CREATE POLICY reconcile_read ON mcp.grants FOR SELECT
    USING (current_setting('app.relay', true) = 'on' AND status = 'revoked' AND revocation_propagated_at IS NULL);
