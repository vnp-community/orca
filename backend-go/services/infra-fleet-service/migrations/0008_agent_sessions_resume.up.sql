ALTER TABLE infra.agent_sessions
  ADD COLUMN resume_provider_session_key TEXT,  -- "session_id" | "conversation_id"
  ADD COLUMN resume_provider_session_id  TEXT;  -- the CLI's OWN id — distinct from agent_sessions.id
