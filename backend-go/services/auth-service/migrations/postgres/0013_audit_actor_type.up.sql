-- Who performed the audited action: a human, an AI agent acting through MCP
-- (BE-MCP-SOL-013) or the system. Existing rows are human/system events.
ALTER TABLE auth.audit_log
  ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'user' CHECK (actor_type IN ('user', 'agent', 'system'));

-- Newest-first keyset paging of agent activity.
CREATE INDEX IF NOT EXISTS idx_audit_log_agent ON auth.audit_log (tenant_id, occurred_at DESC, id DESC) WHERE actor_type = 'agent';
