-- BE-MCP-SOL-014: mirrors the Postgres 0004 (adds mcp_external_secret and the
-- previously missing dev_server_agent_token). Requires MySQL 8.0.19+/TiDB
-- for ALTER TABLE ... DROP CHECK.
ALTER TABLE credential_metadata DROP CHECK credential_metadata_category_check;
ALTER TABLE credential_metadata ADD CONSTRAINT credential_metadata_category_check CHECK (category IN
    ('scm_oauth', 'issue_tracker_oauth', 'ai_provider_key', 'ssh', 'service_secret',
     'dev_server_agent_token', 'mcp_external_secret'));
