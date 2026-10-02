-- MCP personal access tokens (BE-MCP-SOL-006). The signed token itself is a
-- JWT that is never stored; only the SHA-256 of the full secret is kept, for
-- support lookup and reconciliation. Authentication is signature + jti.
CREATE TABLE auth.mcp_tokens (
    jti            TEXT PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    user_id        UUID NOT NULL REFERENCES auth.users (id),
    name           TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    scope          TEXT NOT NULL,
    token_sha256   TEXT NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    first_used_at  TIMESTAMPTZ,
    last_used_at   TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    revoked_by     UUID,
    -- 2160h (not '90 days') so the bound does not depend on the session time zone.
    CONSTRAINT mcp_tokens_max_90d CHECK (expires_at > created_at AND expires_at <= created_at + interval '2160 hours')
);

CREATE INDEX idx_mcp_tokens_user ON auth.mcp_tokens (tenant_id, user_id, created_at DESC);

ALTER TABLE auth.mcp_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.mcp_tokens
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
