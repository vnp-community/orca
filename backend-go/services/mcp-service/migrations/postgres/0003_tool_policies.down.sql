DROP TABLE IF EXISTS mcp.tool_policy_revisions;
DROP TABLE IF EXISTS mcp.tool_policies;
ALTER TABLE mcp.tenant_settings DROP COLUMN IF EXISTS policy_epoch;
