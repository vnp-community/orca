-- MCP personal access tokens (BE-MCP-SOL-006), MySQL/TiDB variant of
-- postgres/0012: no RLS (tenant scoping is enforced in internal/adapter/mysql).
-- Only the SHA-256 of the full secret is stored, never the token.
CREATE TABLE mcp_tokens (
    jti            VARCHAR(64) PRIMARY KEY,
    tenant_id      CHAR(36) NOT NULL,
    user_id        CHAR(36) NOT NULL,
    name           VARCHAR(80) NOT NULL,
    scope          VARCHAR(255) NOT NULL,
    token_sha256   CHAR(64) NOT NULL,
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at     TIMESTAMP(6) NOT NULL,
    first_used_at  TIMESTAMP(6) NULL,
    last_used_at   TIMESTAMP(6) NULL,
    revoked_at     TIMESTAMP(6) NULL,
    revoked_by     CHAR(36) NULL,

    UNIQUE KEY uq_mcp_tokens_sha (token_sha256),
    KEY idx_mcp_tokens_user (tenant_id, user_id, created_at),
    CONSTRAINT mcp_tokens_name_check CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 80),
    CONSTRAINT mcp_tokens_expiry_check CHECK (expires_at > created_at AND expires_at <= created_at + INTERVAL 90 DAY),
    CONSTRAINT fk_mcp_tokens_user FOREIGN KEY (user_id) REFERENCES users (id)
);
