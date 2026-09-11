-- Mirrors postgres/0004_company_email_domains.up.sql. email_domain:
-- TEXT PRIMARY KEY -> VARCHAR(255) PRIMARY KEY — InnoDB requires an
-- explicit key length for a TEXT column used in a PRIMARY KEY/UNIQUE index
-- (3072-byte key limit, no implicit prefix), same translation
-- BE-DB-SOL-005 (annotation-service) used for its own TEXT-PK-adjacent
-- columns. A domain name is well under 255 bytes in practice (DNS labels
-- cap at 253 chars total), so this is a safe narrowing, not a behavior
-- change.
CREATE TABLE company_email_domains (
    email_domain   VARCHAR(255) PRIMARY KEY,
    company_id     CHAR(36) NOT NULL REFERENCES companies(id),
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
CREATE INDEX idx_company_email_domains_company ON company_email_domains (company_id);
