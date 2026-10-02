-- OAuth 2.1 authorization server (BE-MCP-SOL-005), MySQL/TiDB variant of
-- postgres/0011: redirect_uris is JSON instead of TEXT[], no RLS (tenant
-- scoping is enforced in internal/adapter/mysql only). Only SHA-256 hashes of
-- codes and refresh tokens are stored.

-- Deployment-wide registry with no tenant_id: documented exception, see the
-- Postgres variant.
CREATE TABLE oauth_clients (
    client_id       VARCHAR(64) PRIMARY KEY,
    client_name     VARCHAR(100) NOT NULL,
    client_uri      VARCHAR(2048) NULL,
    redirect_uris   JSON NOT NULL,
    registered_via  VARCHAR(8) NOT NULL,
    created_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_used_at    TIMESTAMP(6) NULL,

    CONSTRAINT oauth_clients_via_check CHECK (registered_via IN ('dcr', 'admin')),
    CONSTRAINT oauth_clients_uris_check CHECK (JSON_LENGTH(redirect_uris) BETWEEN 1 AND 5)
);

CREATE TABLE oauth_client_tenant_status (
    tenant_id   CHAR(36) NOT NULL,
    client_id   VARCHAR(64) NOT NULL,
    status      VARCHAR(8) NOT NULL,
    updated_by  CHAR(36) NULL,
    updated_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (tenant_id, client_id),
    CONSTRAINT oauth_status_check CHECK (status IN ('allowed', 'blocked', 'pending')),
    CONSTRAINT fk_oauth_status_client FOREIGN KEY (client_id) REFERENCES oauth_clients (client_id)
);

CREATE TABLE oauth_token_families (
    family_id     CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    user_id       CHAR(36) NOT NULL,
    client_id     VARCHAR(64) NOT NULL,
    grant_id      CHAR(36) NOT NULL, -- logical FK to mcp.grants (no cross-database FK)
    scope         VARCHAR(255) NOT NULL,
    resource      VARCHAR(512) NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    revoked_at    TIMESTAMP(6) NULL,
    revoke_reason VARCHAR(16) NULL,

    CONSTRAINT oauth_families_reason_check CHECK (revoke_reason IS NULL OR revoke_reason IN
        ('user_revoked', 'admin_revoked', 'reuse_detected', 'client_blocked')),
    CONSTRAINT fk_oauth_families_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_oauth_families_client FOREIGN KEY (client_id) REFERENCES oauth_clients (client_id)
);

CREATE TABLE oauth_auth_codes (
    code_hash       VARCHAR(64) PRIMARY KEY,
    family_id       CHAR(36) NOT NULL,
    tenant_id       CHAR(36) NOT NULL,
    redirect_uri    VARCHAR(2048) NOT NULL,
    code_challenge  VARCHAR(128) NOT NULL,
    expires_at      TIMESTAMP(6) NOT NULL,
    used_at         TIMESTAMP(6) NULL,

    CONSTRAINT fk_oauth_codes_family FOREIGN KEY (family_id) REFERENCES oauth_token_families (family_id)
);

CREATE TABLE oauth_refresh_tokens (
    token_hash        VARCHAR(64) PRIMARY KEY,
    family_id         CHAR(36) NOT NULL,
    tenant_id         CHAR(36) NOT NULL,
    expires_at        TIMESTAMP(6) NOT NULL,
    used_at           TIMESTAMP(6) NULL,
    replaced_by_hash  VARCHAR(64) NULL,

    CONSTRAINT fk_oauth_refresh_family FOREIGN KEY (family_id) REFERENCES oauth_token_families (family_id)
);

-- Keyed by (tenant_id, grant_id) so one tenant can never pre-empt another's revocation.
CREATE TABLE oauth_grant_revocations (
    tenant_id   CHAR(36) NOT NULL,
    grant_id    CHAR(36) NOT NULL,
    revoked_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (tenant_id, grant_id)
);

CREATE INDEX idx_oauth_families_user ON oauth_token_families (tenant_id, user_id);
CREATE INDEX idx_oauth_families_grant ON oauth_token_families (grant_id);
CREATE INDEX idx_oauth_families_client ON oauth_token_families (tenant_id, client_id);
CREATE INDEX idx_oauth_codes_expiry ON oauth_auth_codes (expires_at);
CREATE INDEX idx_oauth_refresh_family ON oauth_refresh_tokens (family_id);
