DROP INDEX idx_auth_sessions_refresh_token_hash ON sessions;
ALTER TABLE sessions
    DROP COLUMN refresh_token_hash,
    DROP COLUMN refresh_expires_at;
