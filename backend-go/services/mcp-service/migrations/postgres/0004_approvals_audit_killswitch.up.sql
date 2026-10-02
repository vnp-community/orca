-- Approvals, tool-call journal, kill switches and taint (BE-MCP-SOL-013).
CREATE TABLE mcp.approvals (
    id             UUID PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    user_id        UUID NOT NULL,   -- the only user allowed to decide
    client_id      TEXT NOT NULL,
    client_name    TEXT NOT NULL,
    mcp_session_id TEXT,
    call_id        UUID,
    tool_name      TEXT NOT NULL,
    tool_title     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    risk           TEXT NOT NULL,
    params_hash    TEXT NOT NULL,
    args_preview   TEXT NOT NULL,
    args_redacted  BOOLEAN NOT NULL,
    reasons        TEXT[] NOT NULL DEFAULT '{}',
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'cancelled')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    decided_at     TIMESTAMPTZ,
    decided_via    TEXT CHECK (decided_via IN ('web', 'mobile', 'elicitation')),
    decision_note  TEXT CHECK (char_length(decision_note) <= 500),
    consumed_at    TIMESTAMPTZ,
    consumed_call_id UUID
);
-- Repeating the same call reuses the open approval instead of spamming new ones.
CREATE UNIQUE INDEX uq_approvals_active ON mcp.approvals (tenant_id, user_id, client_id, params_hash)
    WHERE status IN ('pending', 'approved') AND consumed_at IS NULL;
CREATE INDEX idx_approvals_user_status ON mcp.approvals (tenant_id, user_id, status, created_at DESC, id DESC);
CREATE INDEX idx_approvals_expiry ON mcp.approvals (expires_at) WHERE status = 'pending';

CREATE TABLE mcp.kill_switches (
    id        UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    scope     TEXT NOT NULL CHECK (scope IN ('tenant', 'client', 'grant', 'session')),
    target_id TEXT NOT NULL DEFAULT '',
    active    BOOLEAN NOT NULL,
    reason    TEXT NOT NULL CHECK (char_length(reason) BETWEEN 3 AND 500),
    set_by    UUID NOT NULL,
    set_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- post-processing (cancel approvals/calls, revoke refresh tokens) still owed
    cleanup_pending BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (tenant_id, scope, target_id)
);
CREATE INDEX idx_kill_switches_cleanup ON mcp.kill_switches (set_at) WHERE cleanup_pending;

CREATE TABLE mcp.tool_calls (
    id             UUID PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    user_id        UUID NOT NULL,
    client_id      TEXT NOT NULL,
    client_name    TEXT NOT NULL,
    mcp_session_id TEXT,
    root_session_id TEXT,
    tool_name      TEXT NOT NULL,
    channel        TEXT NOT NULL,
    risk           TEXT NOT NULL,
    risk_class     TEXT NOT NULL CHECK (risk_class IN ('read', 'write', 'exec')),
    params_hash    TEXT NOT NULL,
    args_summary   TEXT NOT NULL,
    decision       TEXT NOT NULL CHECK (decision IN ('allow', 'deny', 'approved', 'denied', 'expired')),
    reason_code    TEXT NOT NULL DEFAULT '',
    approval_id    UUID,
    approver       UUID,
    state          TEXT NOT NULL CHECK (state IN ('started', 'done')),
    result         TEXT CHECK (result IN ('ok', 'error')),
    read_untrusted BOOLEAN NOT NULL DEFAULT false,
    duration_ms    BIGINT,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at    TIMESTAMPTZ
);
CREATE INDEX idx_tool_calls_rate ON mcp.tool_calls (tenant_id, user_id, client_id, risk_class, started_at DESC);
CREATE INDEX idx_tool_calls_loop ON mcp.tool_calls (tenant_id, user_id, client_id, params_hash, started_at DESC);
CREATE INDEX idx_tool_calls_stale ON mcp.tool_calls (started_at) WHERE state = 'started';
CREATE INDEX idx_tool_calls_retention ON mcp.tool_calls (started_at);

CREATE TABLE mcp.taint (
    tenant_id     UUID NOT NULL,
    user_id       UUID NOT NULL,
    client_id     TEXT NOT NULL,
    tainted_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, user_id, client_id)
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['approvals', 'kill_switches', 'tool_calls', 'taint'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
        WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;

-- Background workers discover work across tenants (read-only, only rows that
-- still need action), then act inside an ordinary tenant-scoped transaction.
CREATE POLICY worker_read ON mcp.approvals FOR SELECT
    USING (current_setting('app.relay', true) = 'on' AND status = 'pending');
CREATE POLICY worker_read ON mcp.tool_calls FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY worker_read ON mcp.kill_switches FOR SELECT
    USING (current_setting('app.relay', true) = 'on' AND cleanup_pending);
-- Retention purge (MCP_TOOL_CALLS_RETENTION) runs across tenants.
CREATE POLICY worker_purge ON mcp.tool_calls FOR DELETE
    USING (current_setting('app.relay', true) = 'on' AND state = 'done');
