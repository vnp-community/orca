CREATE TABLE sso_group_role_mapping (
    id          CHAR(36) PRIMARY KEY,
    tenant_id   CHAR(36) NOT NULL,
    provider    VARCHAR(32) NOT NULL,   -- domain.SsoProvider values
    group_name  VARCHAR(255) NOT NULL,  -- e.g. "orca-admins" (OIDC) or "org:my-company" (GitHub)
    role        VARCHAR(16) NOT NULL,
    created_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT sso_group_role_mapping_role_check CHECK (role IN ('user', 'admin')),
    UNIQUE (tenant_id, provider, group_name)
);

-- No RLS equivalent — see 0001_init.up.sql's comment.
