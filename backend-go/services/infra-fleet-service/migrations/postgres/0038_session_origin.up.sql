-- BE-MCP-SOL-009 / CONTRACT mcp-ui-api §5: which MCP client session created a
-- terminal or agent session. All columns are NULL for UI-created sessions.
ALTER TABLE infra.terminal_sessions
    ADD COLUMN origin_type           TEXT,
    ADD COLUMN origin_client_name    TEXT,
    ADD COLUMN origin_mcp_session_id TEXT,
    ADD COLUMN origin_user_id        UUID;

-- Reaper lookup: "everything this MCP session created".
CREATE INDEX idx_infra_terminal_sessions_origin_mcp
    ON infra.terminal_sessions (tenant_id, origin_mcp_session_id)
    WHERE origin_mcp_session_id IS NOT NULL;

ALTER TABLE infra.agent_sessions
    ADD COLUMN origin_type           TEXT,
    ADD COLUMN origin_client_name    TEXT,
    ADD COLUMN origin_mcp_session_id TEXT,
    ADD COLUMN origin_user_id        UUID;

CREATE INDEX idx_infra_agent_sessions_origin_mcp
    ON infra.agent_sessions (tenant_id, origin_mcp_session_id)
    WHERE origin_mcp_session_id IS NOT NULL;
