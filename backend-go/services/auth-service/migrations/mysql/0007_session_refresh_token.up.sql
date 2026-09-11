-- CR-RBAC-003/TASK-BE-011: session-refresh rotation fields. Nullable —
-- existing/older sessions and any session Login mints without a refresh
-- token simply have no refresh_token_hash, matching domain.Session's
-- empty-string/zero-time zero value for these fields. VARCHAR(64), not
-- TEXT, for the same UNIQUE-index length-bound reason as
-- sessions.token_hash (also a SHA-256 hex hash).
ALTER TABLE sessions
    ADD COLUMN refresh_token_hash VARCHAR(64),
    ADD COLUMN refresh_expires_at TIMESTAMP(6) NULL;

-- MySQL has no partial unique index (Postgres's `WHERE refresh_token_hash
-- IS NOT NULL`) — but a plain UNIQUE index on a nullable column already
-- treats NULL as distinct-from-every-other-NULL in MySQL/InnoDB (same as
-- Postgres's own default UNIQUE semantics), so multiple sessions with no
-- refresh token don't collide against each other even without the
-- Postgres variant's explicit partial-index workaround.
CREATE UNIQUE INDEX idx_auth_sessions_refresh_token_hash ON sessions (refresh_token_hash);
