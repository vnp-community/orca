-- auth-service's schema is "auth" (see 0001_init.up.sql's "CREATE TABLE
-- auth.users"/"auth.sessions"/"auth.audit_log") — this table follows the
-- same schema-qualified convention, not a bare "users" FK.
--
-- Numbered 0005, not 0004 (TASK-BE-CLI-005's own draft used 0004): a
-- concurrent, unrelated change already claimed 0004
-- (0004_audit_outcome_and_ip.up.sql, auth.audit_log's outcome/ip_address
-- columns) by the time this task ran — verified via `ls migrations/`
-- before writing this file, not assumed from the task doc.
CREATE TABLE auth.issued_service_tokens (
    jti TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth.users(id),
    audience TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_issued_service_tokens_user_id ON auth.issued_service_tokens(user_id);
