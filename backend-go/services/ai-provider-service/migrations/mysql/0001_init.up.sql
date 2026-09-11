-- ai-provider-service owns this database exclusively — no other service
-- reads or writes these tables. MySQL/TiDB variant: no CREATE SCHEMA (a
-- MySQL database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named `ai_provider`,
-- mirroring the Postgres variant's `ai_provider` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md §3's
-- naming convention).
--
-- NO SECRET COLUMNS ANYWHERE IN THIS FILE, BY CONSTRUCTION, NOT CONVENTION.
-- accounts.credential_ref is an opaque pointer that credential-broker-service
-- resolves via its own API — never a DB join, never a plaintext or
-- ciphertext value. Same invariant as the Postgres variant — see that
-- file's comment for the full rationale, unchanged by dialect.
--
-- id has no server-side default here (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): internal/usecase/create_account.go always
-- generates the id in Go (uuid.NewString(), see cmd/server/main.go's
-- usecase.NewCreateAccount(..., uuid.NewString, ...)) before calling
-- Create, so the Postgres default was already dead code from the
-- application's point of view — confirmed the same way BE-DB-SOL-001 §1
-- confirmed it for usage-service's sessions.id. No functional loss.
CREATE TABLE accounts (
    id                   CHAR(36) PRIMARY KEY,
    tenant_id            CHAR(36) NOT NULL,
    provider_type        VARCHAR(32) NOT NULL,
    status               VARCHAR(16) NOT NULL DEFAULT 'pending',
    credential_ref       TEXT NOT NULL,       -- credential-broker-service metadata id, NEVER a secret value
    scope                VARCHAR(16) NOT NULL,
    user_id              CHAR(36),            -- set iff scope = 'user'
    project_id           CHAR(36),            -- set iff scope = 'project'
    rotation_grace_until TIMESTAMP(6) NULL,   -- previous credential_ref stays valid on the agent side until this
    created_at           TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at           TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    -- MySQL 8.0.16+ enforces CHECK constraints (confirmed by running the
    -- integration suite against the mysql:8 test image, same confirmation
    -- basis BE-DB-SOL-006 §1 used for credential-broker-service) — kept as
    -- CHECK rather than downgraded to an unenforced comment.
    CONSTRAINT accounts_provider_type_check CHECK (provider_type IN
        ('anthropic','openai','google','azure','aws_bedrock','ollama','vllm')),
    CONSTRAINT accounts_status_check CHECK (status IN
        ('pending','active','rotating','revoked','error')),
    CONSTRAINT accounts_scope_check CHECK (scope IN ('user','project','server')),
    CONSTRAINT scope_ref_matches_scope CHECK (
        (scope = 'user'    AND user_id IS NOT NULL AND project_id IS NULL) OR
        (scope = 'project' AND project_id IS NOT NULL AND user_id IS NULL) OR
        (scope = 'server'  AND user_id IS NULL AND project_id IS NULL)
    )
);

-- MySQL has no partial index (Postgres's `WHERE scope = '...'` on each of
-- these three) — indexes the full (tenant_id, scope, ...) tuple instead.
-- Each index is smaller in Postgres but still correct (and still
-- selective on tenant_id+scope first) in MySQL — same trade-off
-- annotation-service's idx_annotations_worktree documents (BE-DB-SOL-005).
CREATE INDEX idx_accounts_tenant_scope_user ON accounts (tenant_id, scope, user_id);
CREATE INDEX idx_accounts_tenant_scope_project ON accounts (tenant_id, scope, project_id);
CREATE INDEX idx_accounts_tenant_scope_server ON accounts (tenant_id, scope);
-- idx_accounts_rotating: same partial-index-to-full-index translation —
-- Postgres's `WHERE rotation_grace_until IS NOT NULL` dropped, full
-- (status, rotation_grace_until) pair indexed instead.
CREATE INDEX idx_accounts_rotating ON accounts (status, rotation_grace_until);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. Per
-- BE-DB-SOL-001 §4 (usage-service pilot finding, confirmed by grep: no
-- backend-go code ever calls `SET LOCAL app.tenant_id`, and no migration
-- uses `FORCE ROW LEVEL SECURITY`), the Postgres variant's RLS policy
-- never actually activated either — the pool's connecting role owns the
-- tables and bypasses RLS by default. This migration doesn't regress
-- anything real; it stops pretending a backstop exists that never ran.
-- See TASK-BE-DB-014's tenant-isolation-without-RLS tests for the proof.

-- Aggregate quota/spend rollup — NOT raw usage events (usage-service owns
-- per-session data in a wholly separate database; see
-- ai-provider-service.md §2's bounded-context distinction from usage-service).
--
-- `usage` is backtick-quoted everywhere in this dialect (here and in
-- internal/adapter/mysql/repository.go's queries) — unlike every other
-- table name in this rollout, USAGE is a MySQL reserved word (used in the
-- GRANT ... USAGE privilege grammar), confirmed the hard way: an
-- unquoted `CREATE TABLE usage (...)` fails with a syntax error against a
-- real MySQL 8 server. Postgres has no such reserved word, so
-- `ai_provider.usage` needs no quoting there — this is a genuine,
-- MySQL-only naming footgun, not a stylistic choice.
CREATE TABLE `usage` (
    account_id     CHAR(36) NOT NULL,
    tenant_id      CHAR(36) NOT NULL,
    date           DATE NOT NULL,
    cost_usd       DECIMAL(12,4) NOT NULL DEFAULT 0,
    request_count  BIGINT NOT NULL DEFAULT 0,

    PRIMARY KEY (account_id, date),
    CONSTRAINT fk_usage_account FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE INDEX idx_usage_tenant_date ON `usage` (tenant_id, date DESC);

-- No RLS equivalent here either — same rationale as accounts above.
