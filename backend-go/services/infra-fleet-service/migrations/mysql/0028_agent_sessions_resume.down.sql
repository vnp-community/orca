-- MySQL does not support "DROP COLUMN IF EXISTS" — see migrations/mysql/
-- 0014_connections_grace_period.down.sql's comment.
ALTER TABLE agent_sessions
  DROP COLUMN resume_provider_session_key,
  DROP COLUMN resume_provider_session_id;
