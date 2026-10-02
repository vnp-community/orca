-- MCP sessions and the live SSE stream registry (BE-MCP-SOL-004).
-- secret_hash is SHA-256 of Mcp-Session-Id: the secret itself is never stored.
CREATE TABLE mcp.sessions (
    id               UUID PRIMARY KEY,
    tenant_id        UUID NOT NULL,
    user_id          UUID NOT NULL,
    secret_hash      BYTEA NOT NULL UNIQUE,
    client_id        TEXT NOT NULL DEFAULT '',
    client_name      TEXT NOT NULL DEFAULT '',
    client_version   TEXT NOT NULL DEFAULT '',
    grant_id         TEXT NOT NULL DEFAULT '',
    token_id         TEXT NOT NULL DEFAULT '',
    protocol_version TEXT NOT NULL,
    capabilities     JSONB NOT NULL DEFAULT '{}',
    log_level        TEXT NOT NULL DEFAULT 'warning',
    state            TEXT NOT NULL CHECK (state IN ('initializing', 'ready', 'closed')),
    tool_calls       BIGINT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at        TIMESTAMPTZ,
    close_reason     TEXT
);
CREATE INDEX idx_sessions_tenant_user ON mcp.sessions (tenant_id, user_id) WHERE state <> 'closed';
CREATE INDEX idx_sessions_idle ON mcp.sessions (last_seen_at) WHERE state <> 'closed';

-- A stream is live while heartbeat_at is recent; only standalone GET streams are registered.
CREATE TABLE mcp.session_streams (
    id           UUID PRIMARY KEY,
    session_id   UUID NOT NULL REFERENCES mcp.sessions (id) ON DELETE CASCADE,
    tenant_id    UUID NOT NULL,
    user_id      UUID NOT NULL,
    replica_id   TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('get', 'post')),
    opened_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_session_streams_live ON mcp.session_streams (tenant_id, user_id, heartbeat_at);
CREATE INDEX idx_session_streams_session ON mcp.session_streams (session_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['sessions', 'session_streams'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
        WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;

-- The idle reaper closes sessions across tenants; it opts in per transaction
-- via app.relay (same switch as the outbox relay) and may only SELECT/UPDATE.
CREATE POLICY relay_reap_select ON mcp.sessions FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY relay_reap_update ON mcp.sessions FOR UPDATE
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');
-- Stale stream rows are swept across tenants too.
CREATE POLICY relay_sweep_streams ON mcp.session_streams FOR DELETE
    USING (current_setting('app.relay', true) = 'on');
