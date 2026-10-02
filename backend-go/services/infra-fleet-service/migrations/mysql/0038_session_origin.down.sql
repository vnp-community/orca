DROP INDEX idx_infra_agent_sessions_origin_mcp ON agent_sessions;
ALTER TABLE agent_sessions
    DROP COLUMN origin_user_id,
    DROP COLUMN origin_mcp_session_id,
    DROP COLUMN origin_client_name,
    DROP COLUMN origin_type;

DROP INDEX idx_infra_terminal_sessions_origin_mcp ON terminal_sessions;
ALTER TABLE terminal_sessions
    DROP COLUMN origin_user_id,
    DROP COLUMN origin_mcp_session_id,
    DROP COLUMN origin_client_name,
    DROP COLUMN origin_type;
