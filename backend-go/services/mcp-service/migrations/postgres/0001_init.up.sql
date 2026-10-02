-- mcp-service owns this database exclusively (database-per-service). Other
-- services' ids (tenant, user) are logical references only: no cross-DB FKs.
-- Tables for sessions/OAuth/grants/policies/approvals/external servers are
-- added by the solutions that own them (expand-only migrations).
CREATE SCHEMA IF NOT EXISTS mcp;

CREATE TABLE mcp.tenant_settings (
    tenant_id             UUID PRIMARY KEY,
    -- DDL-level safety net only: the usecase always inserts `enabled`
    -- explicitly from MCP_TENANT_DEFAULT_ENABLED (D6).
    enabled               BOOLEAN NOT NULL DEFAULT false,
    dcr_enabled           BOOLEAN NOT NULL DEFAULT false,
    max_token_days        INT     NOT NULL DEFAULT 90 CHECK (max_token_days BETWEEN 1 AND 90),
    approval_ttl_seconds  INT     NOT NULL DEFAULT 600 CHECK (approval_ttl_seconds BETWEEN 30 AND 86400),
    kill_switch_active    BOOLEAN NOT NULL DEFAULT false,
    kill_switch_reason    TEXT    NOT NULL DEFAULT '',
    kill_switch_at        TIMESTAMPTZ,
    updated_by            UUID,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- NATS consumer dedupe (arch/08).
CREATE TABLE mcp.processed_events (
    tenant_id    UUID NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id)
);

-- Transactional outbox, same shape as usage.outbox_events.
CREATE TABLE mcp.outbox_events (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    subject      TEXT NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    version      INT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_mcp_outbox_unpublished ON mcp.outbox_events (created_at) WHERE published_at IS NULL;

-- RLS that actually applies: FORCE makes the table owner subject to it too
-- (superusers and BYPASSRLS roles still bypass, so the app role must be
-- neither). NULLIF guards pooled connections where a finished transaction
-- leaves app.tenant_id as '' instead of NULL, which would fail the uuid cast.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['tenant_settings', 'processed_events', 'outbox_events'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
        WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;

-- The outbox relay reads and marks rows across tenants. It opts in per
-- transaction via app.relay and can only SELECT/UPDATE, never INSERT.
CREATE POLICY relay_read ON mcp.outbox_events FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_mark_published ON mcp.outbox_events FOR UPDATE
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');
