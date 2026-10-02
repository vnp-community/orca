-- Tool policies (BE-MCP-SOL-012 section D). Tenant policies are data (read
-- through RLS and passed to Rego as input); the Rego bundle stays static.
ALTER TABLE mcp.tenant_settings ADD COLUMN policy_epoch BIGINT NOT NULL DEFAULT 0;

CREATE TABLE mcp.tool_policies (
    id               UUID PRIMARY KEY,
    tenant_id        UUID NOT NULL,
    version          INT NOT NULL DEFAULT 1,
    match_tool       TEXT,
    match_namespace  TEXT,
    match_client_id  TEXT,
    match_roles      TEXT[],
    match_risk       TEXT CHECK (match_risk IN ('read', 'write_reversible', 'exec', 'destructive', 'admin')),
    decision         TEXT NOT NULL CHECK (decision IN ('allow', 'require_approval', 'deny')),
    note             TEXT CHECK (char_length(note) <= 500),
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by       UUID NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- a policy that matches everything is never valid
    CHECK (num_nonnulls(match_tool, match_namespace, match_risk, match_client_id, match_roles) >= 1),
    CHECK (match_roles IS NULL OR match_roles <@ ARRAY['admin', 'user'])
);
CREATE INDEX idx_tool_policies_tenant ON mcp.tool_policies (tenant_id);

-- Append-only: who changed what, when.
CREATE TABLE mcp.tool_policy_revisions (
    policy_id  UUID NOT NULL,
    tenant_id  UUID NOT NULL,
    version    INT NOT NULL,
    op         TEXT NOT NULL CHECK (op IN ('create', 'update', 'delete')),
    snapshot   JSONB NOT NULL,
    changed_by UUID NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (policy_id, version, op)
);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['tool_policies', 'tool_policy_revisions'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
        WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;
