-- MySQL has no INET type — ip stores the printable address (IPv4/IPv6) as
-- text directly, so internal/adapter/mysql's Repository.GetSessionByTokenHash
-- (etc.) reads it back with a plain SELECT, no host()-equivalent unwrap
-- needed the way postgres/session_repository.go needs `host(ip)` to strip
-- the `/32` netmask suffix Postgres's `::text` cast on INET would add.
ALTER TABLE sessions
  ADD COLUMN last_seen_at TIMESTAMP(6) NULL,
  ADD COLUMN ip           VARCHAR(45),
  ADD COLUMN user_agent   TEXT;

-- Index the reaper's scan predicate. Named differently from 0001's
-- idx_auth_sessions_expires_at (same column) purely to mirror the
-- Postgres variant's own two-migrations-same-column history verbatim —
-- MySQL tolerates the redundant index the same way Postgres does.
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
