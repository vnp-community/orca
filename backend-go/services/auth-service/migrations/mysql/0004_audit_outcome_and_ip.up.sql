-- MySQL has no INET type — ip_address stores the printable address
-- (IPv4/IPv6) as text, mirroring session_repository.go's `ip` column
-- translation in migration 0008. VARCHAR(45) covers the longest IPv6
-- textual form.
ALTER TABLE audit_log
    ADD COLUMN outcome     VARCHAR(16) NOT NULL DEFAULT 'allowed',
    ADD CONSTRAINT audit_log_outcome_check CHECK (outcome IN ('allowed', 'denied')),
    ADD COLUMN ip_address  VARCHAR(45);

CREATE INDEX idx_audit_log_outcome ON audit_log (tenant_id, outcome);
