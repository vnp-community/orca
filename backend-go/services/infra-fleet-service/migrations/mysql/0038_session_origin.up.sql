-- BE-MCP-SOL-009 / CONTRACT mcp-ui-api §5: which MCP client session created a
-- terminal or agent session. All columns are NULL for UI-created sessions.
-- VARCHAR(255) (not TEXT) so origin_mcp_session_id can be indexed.
ALTER TABLE terminal_sessions
    ADD COLUMN origin_type           VARCHAR(32),
    ADD COLUMN origin_client_name    VARCHAR(255),
    ADD COLUMN origin_mcp_session_id VARCHAR(255),
    ADD COLUMN origin_user_id        CHAR(36);

CREATE INDEX idx_infra_terminal_sessions_origin_mcp ON terminal_sessions (tenant_id, origin_mcp_session_id);

ALTER TABLE agent_sessions
    ADD COLUMN origin_type           VARCHAR(32),
    ADD COLUMN origin_client_name    VARCHAR(255),
    ADD COLUMN origin_mcp_session_id VARCHAR(255),
    ADD COLUMN origin_user_id        CHAR(36);

CREATE INDEX idx_infra_agent_sessions_origin_mcp ON agent_sessions (tenant_id, origin_mcp_session_id);
