-- credential-broker-service owns this database exclusively — no other
-- service reads or writes these tables. MySQL/TiDB variant: no CREATE
-- SCHEMA (a MySQL database IS the schema-equivalent isolation unit — this
-- migration assumes DATABASE_DSN already points at a database named
-- `credential`, mirroring the Postgres variant's `credential` schema name;
-- see specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md
-- §3's naming convention).
--
-- NO SECRET COLUMNS, EVER. Every column in credential_metadata is a
-- pointer, a status enum, or a timestamp — never a secret value,
-- ciphertext, or decryption key. Same invariant as the Postgres variant —
-- see that file's comment for the full rationale, unchanged by dialect.
CREATE TABLE credential_metadata (
    id           CHAR(36) PRIMARY KEY,
    tenant_id    CHAR(36) NOT NULL,
    owner_id     CHAR(36) NOT NULL,
    category     VARCHAR(32) NOT NULL,
    status       VARCHAR(16) NOT NULL DEFAULT 'pending',
    -- Pointer only: the Vault KV v2 path this credential's ciphertext lives
    -- under. NEVER a secret value, NEVER ciphertext, NEVER a decryption key
    -- — see the schema-level comment above.
    vault_path   VARCHAR(512) NOT NULL,
    created_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT credential_metadata_category_check CHECK (category IN
        ('scm_oauth', 'issue_tracker_oauth', 'ai_provider_key', 'ssh', 'service_secret')),
    CONSTRAINT credential_metadata_status_check CHECK (status IN
        ('pending', 'active', 'rotating', 'revoked', 'error')),

    -- Each Vault path must map to exactly one metadata row — prevents two
    -- rows silently pointing at the same secret with divergent status.
    CONSTRAINT unique_vault_path UNIQUE (vault_path)
);

CREATE INDEX idx_credential_metadata_tenant_category
    ON credential_metadata (tenant_id, category, status);
CREATE INDEX idx_credential_metadata_owner
    ON credential_metadata (tenant_id, owner_id);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. Per
-- BE-DB-SOL-001 §4 (usage-service pilot finding, confirmed by grep: no
-- backend-go code ever calls `SET LOCAL app.tenant_id`, and no migration
-- uses `FORCE ROW LEVEL SECURITY`), the Postgres variant's RLS policy
-- never actually activated either — the pool's connecting role owns the
-- tables and bypasses RLS by default. This migration doesn't regress
-- anything real; it stops pretending a backstop exists that never ran.
-- See TASK-BE-DB-011's tenant-isolation-without-RLS test for the proof.

-- Append-only. No secret values here either — a row records THAT an access
-- happened, by whom, and which operation, never the value accessed.
CREATE TABLE access_audit_log (
    id                BIGINT AUTO_INCREMENT PRIMARY KEY,
    credential_id     CHAR(36) NOT NULL,
    accessor_service  VARCHAR(128) NOT NULL,  -- resolved from mTLS/JWT identity, never client-asserted
    action            VARCHAR(16) NOT NULL,
    occurred_at       TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT access_audit_log_action_check CHECK (action IN ('write', 'resolve', 'rotate', 'revoke')),
    CONSTRAINT fk_access_audit_credential FOREIGN KEY (credential_id) REFERENCES credential_metadata (id)
);

CREATE INDEX idx_access_audit_credential ON access_audit_log (credential_id, occurred_at DESC);
CREATE INDEX idx_access_audit_service ON access_audit_log (accessor_service, occurred_at DESC);

-- No UPDATE/DELETE grants on this table for the service's own DB role beyond
-- INSERT/SELECT — enforced at the database role level in a real deployment,
-- not just by application code discipline (see credential-broker-service.md
-- §8, §9). This scaffold's migration does not itself create/grant that
-- restricted role — see this service's README "Known gaps", same as the
-- Postgres variant.
