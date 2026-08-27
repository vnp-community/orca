-- auth-service owns this database exclusively — no other service reads or
-- writes these tables. See specs/backend-go/architecture/05-data-architecture.md
-- and specs/backend-go/services/auth-service.md §5.
CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.users (
    id              UUID PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    email           TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('user', 'admin')),
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_auth_users_status ON auth.users (tenant_id, is_active);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE auth.users ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.users
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- token_hash is a SHA-256 hash (hex) of the opaque session token — the raw
-- token is never stored, only returned to the caller once at Login. See
-- domain.Session's doc comment and auth-service.md §5/§9.
CREATE TABLE auth.sessions (
    token_hash      TEXT PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES auth.users (id),
    tenant_id       UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX idx_auth_sessions_user_id ON auth.sessions (user_id);
CREATE INDEX idx_auth_sessions_expires_at ON auth.sessions (expires_at); -- for the reaper job, see README "Known gaps"

ALTER TABLE auth.sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.sessions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Append-only: this service's own Postgres role should be granted
-- INSERT/SELECT but not UPDATE/DELETE on auth.audit_log in production, per
-- auth-service.md §9 ("Audit log integrity"). Not expressed as a REVOKE
-- here because the deploying role/migration user isn't this service's
-- runtime role in every environment — wire the grant in the environment's
-- provisioning step, not this migration.
CREATE TABLE auth.audit_log (
    id              UUID PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    actor_id        UUID, -- nullable: empty for system-initiated events, see domain.AuditEntry
    action          TEXT NOT NULL,
    target          TEXT NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- BRIN suits an append-only, time-ordered table per auth-service.md §5.
CREATE INDEX idx_auth_audit_log_occurred_at ON auth.audit_log USING BRIN (occurred_at);
CREATE INDEX idx_auth_audit_log_actor ON auth.audit_log (actor_id);

ALTER TABLE auth.audit_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.audit_log
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
