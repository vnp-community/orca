-- Maps an email domain (e.g. "vnpay.vn") to the ONE company whose users
-- authenticate with that domain — resolves which tenant a brand-new SSO
-- signup belongs to (auth-service's ResolveCompanyByEmailDomain call,
-- CR-LOGIN-001 multi-tenant follow-up). A domain can belong to only one
-- company (PRIMARY KEY on email_domain itself, always stored lowercased —
-- see domain.NormalizeEmailDomain); a company may register multiple
-- domains (e.g. vnpay.vn AND vnpay.com.vn), hence no UNIQUE on company_id.
CREATE TABLE tenant.company_email_domains (
    email_domain   TEXT PRIMARY KEY,
    company_id     UUID NOT NULL REFERENCES tenant.companies(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_company_email_domains_company ON tenant.company_email_domains (company_id);
