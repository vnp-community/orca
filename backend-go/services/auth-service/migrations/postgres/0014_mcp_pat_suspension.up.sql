-- Tenant-level suspension of MCP personal access tokens, driven by the MCP
-- kill switch (BE-MCP-SOL-013). A row suspends every PAT of the tenant without
-- touching the tokens, so deleting it restores them; revoked tokens stay revoked.
CREATE TABLE auth.mcp_pat_suspensions (
    tenant_id     UUID PRIMARY KEY,
    suspended_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason        TEXT NOT NULL DEFAULT ''
);

ALTER TABLE auth.mcp_pat_suspensions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.mcp_pat_suspensions
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
