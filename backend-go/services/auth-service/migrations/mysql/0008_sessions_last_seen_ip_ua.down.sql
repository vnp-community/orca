DROP INDEX idx_sessions_expires_at ON sessions;

ALTER TABLE sessions
  DROP COLUMN last_seen_at,
  DROP COLUMN ip,
  DROP COLUMN user_agent;
