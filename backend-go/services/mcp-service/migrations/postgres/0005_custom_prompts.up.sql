-- Tenant custom prompts (BE-MCP-SOL-011). Soft delete + partial unique index
-- so a deleted prompt's name can be reused; version is the optimistic-lock token.
CREATE TABLE mcp.custom_prompts (
    id          UUID PRIMARY KEY,
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL CHECK (name ~ '^[a-z][a-z0-9_]{2,47}$'),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    arguments   JSONB NOT NULL DEFAULT '[]',
    template    TEXT NOT NULL CHECK (char_length(template) BETWEEN 1 AND 8192),
    version     INTEGER NOT NULL DEFAULT 1,
    created_by  UUID NOT NULL,
    updated_by  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_custom_prompts_tenant_name ON mcp.custom_prompts (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_custom_prompts_tenant ON mcp.custom_prompts (tenant_id) WHERE deleted_at IS NULL;

ALTER TABLE mcp.custom_prompts ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.custom_prompts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mcp.custom_prompts
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
