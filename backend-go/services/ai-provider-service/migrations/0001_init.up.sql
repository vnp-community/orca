-- ai-provider-service owns this database exclusively — no other service
-- reads or writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
--
-- NO SECRET COLUMNS ANYWHERE IN THIS FILE, BY CONSTRUCTION, NOT CONVENTION.
-- ai_provider.accounts.credential_ref is an opaque pointer that
-- credential-broker-service resolves via its own API — never a DB join,
-- never a plaintext or ciphertext value. A full dump of every table in this
-- schema must never yield a usable credential. See
-- specs/backend-go/services/ai-provider-service.md §1, §5, §9.
CREATE SCHEMA IF NOT EXISTS ai_provider;

CREATE TABLE ai_provider.accounts (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID NOT NULL,
    provider_type        TEXT NOT NULL CHECK (provider_type IN
                             ('anthropic','openai','google','azure','aws_bedrock','ollama','vllm')),
    status               TEXT NOT NULL DEFAULT 'pending' CHECK (status IN
                             ('pending','active','rotating','revoked','error')),
    credential_ref       TEXT NOT NULL,       -- credential-broker-service metadata id, NEVER a secret value
    scope                TEXT NOT NULL CHECK (scope IN ('user','project','server')),
    user_id              UUID,                -- set iff scope = 'user'
    project_id           UUID,                -- set iff scope = 'project'
    rotation_grace_until TIMESTAMPTZ,          -- previous credential_ref stays valid on the agent side until this
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT scope_ref_matches_scope CHECK (
        (scope = 'user'    AND user_id IS NOT NULL AND project_id IS NULL) OR
        (scope = 'project' AND project_id IS NOT NULL AND user_id IS NULL) OR
        (scope = 'server'  AND user_id IS NULL AND project_id IS NULL)
    )
);

CREATE INDEX idx_accounts_tenant_scope_user ON ai_provider.accounts (tenant_id, scope, user_id)
    WHERE scope = 'user';
CREATE INDEX idx_accounts_tenant_scope_project ON ai_provider.accounts (tenant_id, scope, project_id)
    WHERE scope = 'project';
CREATE INDEX idx_accounts_tenant_scope_server ON ai_provider.accounts (tenant_id, scope)
    WHERE scope = 'server';
CREATE INDEX idx_accounts_rotating ON ai_provider.accounts (status, rotation_grace_until)
    WHERE rotation_grace_until IS NOT NULL;

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE ai_provider.accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_provider.accounts
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Aggregate quota/spend rollup — NOT raw usage events (usage-service owns
-- per-session data in a wholly separate database; see
-- ai-provider-service.md §2's bounded-context distinction from usage-service).
CREATE TABLE ai_provider.usage (
    account_id     UUID NOT NULL REFERENCES ai_provider.accounts(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL,
    date           DATE NOT NULL,
    cost_usd       NUMERIC(12,4) NOT NULL DEFAULT 0,
    request_count  BIGINT NOT NULL DEFAULT 0,

    PRIMARY KEY (account_id, date)
);

CREATE INDEX idx_usage_tenant_date ON ai_provider.usage (tenant_id, date DESC);

ALTER TABLE ai_provider.usage ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_provider.usage
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
