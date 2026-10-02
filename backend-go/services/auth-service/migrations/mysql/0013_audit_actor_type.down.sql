DROP INDEX idx_audit_log_actor_type_time ON audit_log;
ALTER TABLE audit_log DROP CONSTRAINT audit_log_actor_type_check, DROP COLUMN actor_type;
