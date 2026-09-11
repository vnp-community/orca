DROP INDEX idx_audit_log_outcome ON audit_log;
ALTER TABLE audit_log DROP CONSTRAINT audit_log_outcome_check;
ALTER TABLE audit_log DROP COLUMN outcome, DROP COLUMN ip_address;
