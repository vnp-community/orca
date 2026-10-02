-- OAuth 2.1 authorization server (BE-MCP-SOL-005). Only mcp-service issues
-- authorization codes, and only the SHA-256 hash of a code or refresh token
-- is ever stored.

-- Deployment-wide registry: dynamic client registration is anonymous, so no
-- tenant is known yet. This is the documented exception to the "tenant_id
-- NOT NULL on every table" rule: the table holds only app identity, never
-- tenant data. Per-tenant allow/block state is oauth_client_tenant_status.
CREATE TABLE auth.oauth_clients (
    client_id       TEXT PRIMARY KEY,
    client_name     TEXT NOT NULL CHECK (char_length(client_name) BETWEEN 1 AND 100),
    client_uri      TEXT,
    redirect_uris   TEXT[] NOT NULL CHECK (cardinality(redirect_uris) BETWEEN 1 AND 5),
    registered_via  TEXT NOT NULL CHECK (registered_via IN ('dcr', 'admin')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ
);

CREATE TABLE auth.oauth_client_tenant_status (
    tenant_id   UUID NOT NULL,
    client_id   TEXT NOT NULL REFERENCES auth.oauth_clients (client_id),
    status      TEXT NOT NULL CHECK (status IN ('allowed', 'blocked', 'pending')),
    updated_by  UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, client_id)
);

-- One family = one successful authorization (code + every rotated refresh
-- token + the access tokens minted from them).
CREATE TABLE auth.oauth_token_families (
    family_id     UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    user_id       UUID NOT NULL REFERENCES auth.users (id),
    client_id     TEXT NOT NULL REFERENCES auth.oauth_clients (client_id),
    grant_id      UUID NOT NULL, -- logical FK to mcp.grants (no cross-database FK)
    scope         TEXT NOT NULL,
    resource      TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ,
    revoke_reason TEXT CHECK (revoke_reason IS NULL OR revoke_reason IN
        ('user_revoked', 'admin_revoked', 'reuse_detected', 'client_blocked'))
);

CREATE TABLE auth.oauth_auth_codes (
    code_hash       TEXT PRIMARY KEY,
    family_id       UUID NOT NULL REFERENCES auth.oauth_token_families (family_id),
    tenant_id       UUID NOT NULL,
    redirect_uri    TEXT NOT NULL,
    code_challenge  TEXT NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ
);

CREATE TABLE auth.oauth_refresh_tokens (
    token_hash        TEXT PRIMARY KEY,
    family_id         UUID NOT NULL REFERENCES auth.oauth_token_families (family_id),
    tenant_id         UUID NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    used_at           TIMESTAMPTZ, -- set when rotated: presenting it again is reuse
    replaced_by_hash  TEXT
);

-- Keyed by (tenant_id, grant_id) so one tenant can never pre-empt another's revocation.
CREATE TABLE auth.oauth_grant_revocations (
    tenant_id   UUID NOT NULL,
    grant_id    UUID NOT NULL,
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, grant_id)
);

CREATE INDEX idx_oauth_families_user ON auth.oauth_token_families (tenant_id, user_id);
CREATE INDEX idx_oauth_families_grant ON auth.oauth_token_families (grant_id);
CREATE INDEX idx_oauth_families_client ON auth.oauth_token_families (tenant_id, client_id);
CREATE INDEX idx_oauth_codes_expiry ON auth.oauth_auth_codes (expires_at); -- reaper
CREATE INDEX idx_oauth_refresh_family ON auth.oauth_refresh_tokens (family_id);

-- Same RLS shape as 0001_init. /token looks rows up by hash with no tenant in
-- context, so (as for sessions) the tenant is read from the row and the
-- application layer is the primary enforcement.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['oauth_client_tenant_status', 'oauth_token_families', 'oauth_auth_codes',
                           'oauth_refresh_tokens', 'oauth_grant_revocations'] LOOP
    EXECUTE format('ALTER TABLE auth.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON auth.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;
