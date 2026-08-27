-- credential-broker-service owns this database exclusively — no other
-- service reads or writes these tables. See
-- specs/backend-go/architecture/05-data-architecture.md.
--
-- NO SECRET COLUMNS, EVER. Every column in credential.credential_metadata is
-- a pointer, a status enum, or a timestamp — never a secret value,
-- ciphertext, or decryption key. If a future migration ever adds a column
-- that could hold secret material (a raw token, a key, ciphertext, even a
-- hash of a secret used for anything beyond non-reversible integrity
-- checking), that is a design violation of this service's entire reason for
-- existing (see credential-broker-service.md §5), not a normal schema
-- evolution.
CREATE SCHEMA IF NOT EXISTS credential;

CREATE TABLE credential.credential_metadata (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    owner_id     UUID NOT NULL,
    category     TEXT NOT NULL CHECK (category IN
                     ('scm_oauth', 'issue_tracker_oauth', 'ai_provider_key', 'ssh', 'service_secret')),
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN
                     ('pending', 'active', 'rotating', 'revoked', 'error')),
    -- Pointer only: the Vault KV v2 path this credential's ciphertext lives
    -- under. NEVER a secret value, NEVER ciphertext, NEVER a decryption key
    -- — see the schema-level comment above.
    vault_path   TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Each Vault path must map to exactly one metadata row — prevents two
    -- rows silently pointing at the same secret with divergent status.
    CONSTRAINT unique_vault_path UNIQUE (vault_path)
);

CREATE INDEX idx_credential_metadata_tenant_category
    ON credential.credential_metadata (tenant_id, category, status);
CREATE INDEX idx_credential_metadata_owner
    ON credential.credential_metadata (tenant_id, owner_id);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see
-- internal/adapter/postgres) is the primary enforcement; this is the
-- secondary backstop.
ALTER TABLE credential.credential_metadata ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON credential.credential_metadata
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Append-only. No secret values here either — a row records THAT an access
-- happened, by whom, and which operation, never the value accessed.
CREATE TABLE credential.access_audit_log (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    credential_id     UUID NOT NULL REFERENCES credential.credential_metadata (id),
    accessor_service  TEXT NOT NULL,  -- resolved from mTLS/JWT identity, never client-asserted
    action            TEXT NOT NULL CHECK (action IN ('write', 'resolve', 'rotate', 'revoke')),
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_access_audit_credential ON credential.access_audit_log (credential_id, occurred_at DESC);
CREATE INDEX idx_access_audit_service ON credential.access_audit_log (accessor_service, occurred_at DESC);

-- No UPDATE/DELETE grants on this table for the service's own DB role beyond
-- INSERT/SELECT — enforced at the Postgres role level in a real deployment,
-- not just by application code discipline (see credential-broker-service.md
-- §8, §9). This scaffold's migration does not itself create/grant that
-- restricted role — see this service's README "Known gaps".
