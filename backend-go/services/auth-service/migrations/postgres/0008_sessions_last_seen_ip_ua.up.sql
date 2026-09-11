ALTER TABLE auth.sessions
  ADD COLUMN last_seen_at TIMESTAMPTZ,
  ADD COLUMN ip           INET,
  ADD COLUMN user_agent   TEXT;

-- Index the reaper's scan predicate.
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON auth.sessions (expires_at);
