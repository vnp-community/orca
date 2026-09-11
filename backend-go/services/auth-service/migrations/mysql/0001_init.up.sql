-- auth-service owns this database exclusively — no other service reads or
-- writes these tables. MySQL/TiDB variant: no CREATE SCHEMA (a MySQL
-- database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named `auth`,
-- mirroring the Postgres variant's `auth` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md §3's
-- naming convention).
CREATE TABLE users (
    id              CHAR(36) PRIMARY KEY,
    tenant_id       CHAR(36) NOT NULL,
    email           VARCHAR(320) NOT NULL, -- RFC 5321 max mailbox length; TEXT can't carry a UNIQUE index in InnoDB without a prefix length
    name            TEXT NOT NULL,
    password_hash   TEXT NOT NULL,
    role            VARCHAR(16) NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT users_role_check CHECK (role IN ('user', 'admin')),
    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_auth_users_status ON users (tenant_id, is_active);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/*.go is the ONLY enforcement
-- mechanism here, not a secondary backstop. Per BE-DB-SOL-001 §4
-- (usage-service pilot finding, confirmed by grep: no backend-go code ever
-- calls `SET LOCAL app.tenant_id`, and no migration uses `FORCE ROW LEVEL
-- SECURITY`), the Postgres variant's RLS policy below never actually
-- activated either — the pool's connecting role owns the tables and
-- bypasses RLS by default. This migration doesn't regress anything real;
-- it stops pretending a backstop exists that never ran:
--
--   ALTER TABLE auth.users ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON auth.users
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- token_hash is a SHA-256 hash (hex, 64 chars) of the opaque session token
-- — the raw token is never stored, only returned to the caller once at
-- Login. VARCHAR(64) (not TEXT): a fixed-length hash value, and MySQL
-- requires an explicit, bounded length for a PRIMARY KEY column anyway
-- (InnoDB's key-part limit) — see domain.Session's doc comment and
-- auth-service.md §5/§9.
CREATE TABLE sessions (
    token_hash      VARCHAR(64) PRIMARY KEY,
    user_id         CHAR(36) NOT NULL,
    tenant_id       CHAR(36) NOT NULL,
    created_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at      TIMESTAMP(6) NOT NULL,
    revoked_at      TIMESTAMP(6) NULL,

    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX idx_auth_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_auth_sessions_expires_at ON sessions (expires_at); -- for the reaper job, see README "Known gaps"

-- No RLS equivalent — see users' comment above.

-- Append-only: this service's own MySQL role should be granted
-- INSERT/SELECT but not UPDATE/DELETE on audit_log in production, per
-- auth-service.md §9 ("Audit log integrity"). Not expressed as a REVOKE
-- here for the same reason the Postgres variant doesn't — wire the grant
-- in the environment's provisioning step, not this migration.
CREATE TABLE audit_log (
    id              CHAR(36) PRIMARY KEY,
    tenant_id       CHAR(36) NOT NULL,
    actor_id        CHAR(36), -- nullable: empty for system-initiated events, see domain.AuditEntry
    action          VARCHAR(255) NOT NULL,
    target          TEXT NOT NULL,
    occurred_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

-- MySQL/InnoDB has no BRIN index type (Postgres's append-only,
-- time-ordered optimization) — a regular B-tree index on occurred_at is
-- the closest equivalent, larger on disk but functionally correct for the
-- same range-scan query shapes.
CREATE INDEX idx_auth_audit_log_occurred_at ON audit_log (occurred_at);
CREATE INDEX idx_auth_audit_log_actor ON audit_log (actor_id);

-- No RLS equivalent — see users' comment above.
