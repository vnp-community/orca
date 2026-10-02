DROP INDEX IF EXISTS auth.idx_audit_log_agent;
ALTER TABLE auth.audit_log DROP COLUMN IF EXISTS actor_type;
