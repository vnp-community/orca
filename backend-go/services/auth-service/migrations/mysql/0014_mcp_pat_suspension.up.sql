-- Tenant-level MCP PAT suspension (MySQL/TiDB variant of postgres/0014): no RLS,
-- tenant scoping is enforced in internal/adapter/mysql.
CREATE TABLE mcp_pat_suspensions (
    tenant_id     CHAR(36) PRIMARY KEY,
    suspended_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    reason        VARCHAR(500) NOT NULL DEFAULT ''
);
