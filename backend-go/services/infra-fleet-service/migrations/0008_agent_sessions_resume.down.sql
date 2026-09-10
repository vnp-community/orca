ALTER TABLE infra.agent_sessions
  DROP COLUMN IF EXISTS resume_provider_session_key,
  DROP COLUMN IF EXISTS resume_provider_session_id;
