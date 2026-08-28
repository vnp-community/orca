DROP INDEX IF EXISTS auth.idx_audit_log_target;
DROP INDEX IF EXISTS auth.idx_audit_log_action;

ALTER TABLE auth.audit_log
  DROP COLUMN IF EXISTS ip_address,
  DROP COLUMN IF EXISTS metadata,
  DROP COLUMN IF EXISTS target_id,
  DROP COLUMN IF EXISTS target_type;
