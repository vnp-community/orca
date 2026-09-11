DROP INDEX IF EXISTS auth.idx_sessions_expires_at;

ALTER TABLE auth.sessions
  DROP COLUMN IF EXISTS last_seen_at,
  DROP COLUMN IF EXISTS ip,
  DROP COLUMN IF EXISTS user_agent;
