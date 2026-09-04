-- sso_provider is the provider this user most recently authenticated
-- through — display-only ("signed in via GitHub"), updated on every SSO
-- login (see UserRepository.SetSsoProvider); NULL for an account that has
-- only ever logged in with a local password.
ALTER TABLE auth.users ADD COLUMN sso_provider TEXT
    CHECK (sso_provider IS NULL OR sso_provider IN ('github', 'google', 'oidc'));

-- CR-LOGIN-001: links an external IdP identity (GitHub/Google/generic OIDC)
-- to exactly one auth.users row. Looked up FIRST on every SSO login (by
-- provider + external_subject) before any email-based logic runs — see
-- internal/usecase/login_or_provision_sso_user.go's doc comment.
CREATE TABLE auth.sso_identities (
    id                UUID PRIMARY KEY,
    user_id           UUID NOT NULL REFERENCES auth.users (id),
    tenant_id         UUID NOT NULL,
    provider          TEXT NOT NULL CHECK (provider IN ('github', 'google', 'oidc')),
    external_subject  TEXT NOT NULL, -- IdP's stable subject: GitHub numeric user id (as text) or OIDC "sub" claim
    email_at_link     TEXT NOT NULL DEFAULT '', -- audit trail only, never re-read for auth decisions
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at     TIMESTAMPTZ,

    -- One IdP identity maps to exactly one local user, forever.
    UNIQUE (provider, external_subject)
);

CREATE INDEX idx_auth_sso_identities_user_id ON auth.sso_identities (user_id);

ALTER TABLE auth.sso_identities ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON auth.sso_identities
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
