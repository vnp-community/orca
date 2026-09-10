DROP INDEX IF EXISTS auth.idx_audit_log_target;
DROP INDEX IF EXISTS auth.idx_audit_log_action;

-- ip_address is dropped by 0004_audit_outcome_and_ip's down migration.
ALTER TABLE auth.audit_log
  DROP COLUMN IF EXISTS metadata,
  DROP COLUMN IF EXISTS target_id,
  DROP COLUMN IF EXISTS target_type;
