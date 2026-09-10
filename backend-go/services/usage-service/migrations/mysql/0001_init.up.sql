-- usage-service owns this database exclusively — no other service reads or
-- writes these tables. MySQL/TiDB variant: no CREATE SCHEMA (a MySQL
-- database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named `usage`,
-- mirroring the Postgres variant's `usage` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md).
CREATE TABLE sessions (
    id                  VARCHAR(64) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    user_id             CHAR(36) NOT NULL,
    provider            VARCHAR(16) NOT NULL,
    worktree_id         VARCHAR(64) NOT NULL DEFAULT '',
    input_tokens        BIGINT NOT NULL DEFAULT 0,
    output_tokens       BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens   BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens  BIGINT NOT NULL DEFAULT 0,
    cost_usd            DOUBLE NOT NULL DEFAULT 0,
    started_at          TIMESTAMP(6) NOT NULL,
    ended_at            TIMESTAMP(6) NULL,
    request_id          VARCHAR(128) NOT NULL,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT provider_check CHECK (provider IN ('claude', 'codex', 'opencode')),
    CONSTRAINT input_tokens_check CHECK (input_tokens >= 0),
    CONSTRAINT output_tokens_check CHECK (output_tokens >= 0),
    CONSTRAINT cache_read_tokens_check CHECK (cache_read_tokens >= 0),
    CONSTRAINT cache_write_tokens_check CHECK (cache_write_tokens >= 0),

    UNIQUE KEY uniq_tenant_request (tenant_id, request_id)
);

CREATE INDEX idx_sessions_tenant_user ON sessions (tenant_id, user_id, started_at DESC);
CREATE INDEX idx_sessions_tenant_started ON sessions (tenant_id, started_at DESC);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. See
-- TASK-BE-DB-003's finding: this was already true for the Postgres
-- adapter too (no code ever calls SET LOCAL app.tenant_id, so RLS never
-- actually activated there either) — this migration doesn't regress
-- anything, it just stops pretending a backstop exists that never ran.

CREATE TABLE daily_rollups (
    tenant_id            CHAR(36) NOT NULL,
    user_id              CHAR(36) NOT NULL,
    provider             VARCHAR(16) NOT NULL,
    day                  DATE NOT NULL,
    total_input_tokens   BIGINT NOT NULL DEFAULT 0,
    total_output_tokens  BIGINT NOT NULL DEFAULT 0,
    total_cost_usd       DOUBLE NOT NULL DEFAULT 0,
    session_count        BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT daily_rollups_provider_check CHECK (provider IN ('claude', 'codex', 'opencode')),
    PRIMARY KEY (tenant_id, user_id, provider, day)
);
