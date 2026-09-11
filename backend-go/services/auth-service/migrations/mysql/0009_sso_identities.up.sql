-- sso_provider is the provider this user most recently authenticated
-- through — display-only ("signed in via GitHub"), updated on every SSO
-- login (see UserRepository.SetSsoProvider); NULL for an account that has
-- only ever logged in with a local password.
ALTER TABLE users ADD COLUMN sso_provider VARCHAR(16);
ALTER TABLE users ADD CONSTRAINT users_sso_provider_check
    CHECK (sso_provider IS NULL OR sso_provider IN ('github', 'google', 'oidc'));

-- CR-LOGIN-001: links an external IdP identity (GitHub/Google/generic OIDC)
-- to exactly one users row. Looked up FIRST on every SSO login (by
-- provider + external_subject) before any email-based logic runs — see
-- internal/usecase/login_or_provision_sso_user.go's doc comment.
CREATE TABLE sso_identities (
    id                CHAR(36) PRIMARY KEY,
    user_id           CHAR(36) NOT NULL,
    tenant_id         CHAR(36) NOT NULL,
    provider          VARCHAR(16) NOT NULL,
    external_subject  VARCHAR(255) NOT NULL, -- IdP's stable subject: GitHub numeric user id (as text) or OIDC "sub" claim
    email_at_link     VARCHAR(320) NOT NULL DEFAULT '', -- audit trail only, never re-read for auth decisions
    created_at        TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_login_at     TIMESTAMP(6) NULL,

    CONSTRAINT sso_identities_provider_check CHECK (provider IN ('github', 'google', 'oidc')),
    CONSTRAINT fk_sso_identities_user FOREIGN KEY (user_id) REFERENCES users (id),
    -- One IdP identity maps to exactly one local user, forever.
    UNIQUE (provider, external_subject)
);

CREATE INDEX idx_auth_sso_identities_user_id ON sso_identities (user_id);

-- No RLS equivalent — see 0001_init.up.sql's comment.
