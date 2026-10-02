DROP INDEX IF EXISTS infra.idx_infra_agent_sessions_origin_mcp;
ALTER TABLE infra.agent_sessions
    DROP COLUMN origin_user_id,
    DROP COLUMN origin_mcp_session_id,
    DROP COLUMN origin_client_name,
    DROP COLUMN origin_type;

DROP INDEX IF EXISTS infra.idx_infra_terminal_sessions_origin_mcp;
ALTER TABLE infra.terminal_sessions
    DROP COLUMN origin_user_id,
    DROP COLUMN origin_mcp_session_id,
    DROP COLUMN origin_client_name,
    DROP COLUMN origin_type;
