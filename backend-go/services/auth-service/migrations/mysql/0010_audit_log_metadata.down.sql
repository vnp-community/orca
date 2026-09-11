DROP INDEX idx_audit_log_target ON audit_log;
DROP INDEX idx_audit_log_action ON audit_log;

-- ip_address is dropped by 0004_audit_outcome_and_ip's down migration.
ALTER TABLE audit_log
  DROP COLUMN metadata,
  DROP COLUMN target_id,
  DROP COLUMN target_type;
