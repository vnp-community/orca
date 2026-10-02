-- BE-MCP-SOL-014: allow the new `mcp_external_secret` category. This also
-- repairs a pre-existing gap: `dev_server_agent_token` (domain category and
-- proto value 6) was never added to the 0001 CHECK, so writing it failed.
ALTER TABLE credential.credential_metadata DROP CONSTRAINT IF EXISTS credential_metadata_category_check;
ALTER TABLE credential.credential_metadata ADD CONSTRAINT credential_metadata_category_check CHECK (category IN
    ('scm_oauth', 'issue_tracker_oauth', 'ai_provider_key', 'ssh', 'service_secret',
     'dev_server_agent_token', 'mcp_external_secret'));
