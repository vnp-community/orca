-- usage-service owns this database exclusively — no other service reads or
-- writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
CREATE SCHEMA IF NOT EXISTS usage;

-- Unified sessions table across providers (Claude/Codex/OpenCode) with a
-- discriminator column, rather than 3 near-duplicate table sets — see
-- usage-service.md §5's rationale.
CREATE TABLE usage.sessions (
    id                  TEXT PRIMARY KEY,
    tenant_id           UUID NOT NULL,
    user_id             UUID NOT NULL,
    provider            TEXT NOT NULL CHECK (provider IN ('claude', 'codex', 'opencode')),
    worktree_id         TEXT NOT NULL DEFAULT '',
    input_tokens        BIGINT NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens       BIGINT NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    cache_read_tokens   BIGINT NOT NULL DEFAULT 0 CHECK (cache_read_tokens >= 0),
    cache_write_tokens  BIGINT NOT NULL DEFAULT 0 CHECK (cache_write_tokens >= 0),
    cost_usd            DOUBLE PRECISION NOT NULL DEFAULT 0,
    started_at          TIMESTAMPTZ NOT NULL,
    ended_at            TIMESTAMPTZ,
    request_id          TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (tenant_id, request_id) -- idempotency, see standards/api-design-guidelines.md
);

CREATE INDEX idx_usage_sessions_tenant_user ON usage.sessions (tenant_id, user_id, started_at DESC);
CREATE INDEX idx_usage_sessions_tenant_started ON usage.sessions (tenant_id, started_at DESC);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE usage.sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON usage.sessions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE usage.daily_rollups (
    tenant_id            UUID NOT NULL,
    user_id              UUID NOT NULL,
    provider             TEXT NOT NULL CHECK (provider IN ('claude', 'codex', 'opencode')),
    day                  DATE NOT NULL,
    total_input_tokens   BIGINT NOT NULL DEFAULT 0,
    total_output_tokens  BIGINT NOT NULL DEFAULT 0,
    total_cost_usd       DOUBLE PRECISION NOT NULL DEFAULT 0,
    session_count        BIGINT NOT NULL DEFAULT 0,

    PRIMARY KEY (tenant_id, user_id, provider, day)
);

ALTER TABLE usage.daily_rollups ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON usage.daily_rollups
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
