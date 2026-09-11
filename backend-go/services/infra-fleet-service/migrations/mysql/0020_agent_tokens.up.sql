-- Persistent, named, per-DevServer agent tokens (BL-AWS-03). Coexists with
-- (does not replace) the ephemeral bootstrap Registry/TokenIssuer in
-- adapter/agentwsserver — see usecase/create_agent_token.go's doc comment
-- for how the two are reconciled at handshake time.
CREATE TABLE agent_tokens (
    id                CHAR(36) PRIMARY KEY,
    tenant_id         CHAR(36) NOT NULL,
    dev_server_id     CHAR(36) NOT NULL REFERENCES dev_servers(id),
    name              TEXT NOT NULL,
    -- Exactly one of token_hash / credential_ref_id is set, depending on
    -- the owning dev_server's connection_mode — see SOL-AWS-01 for why
    -- relay-websocket's row can't be a bare hash (Orca must itself present
    -- the plaintext outbound, so that case's secret lives in
    -- credential-broker-service/Vault, referenced here by id only).
    -- VARCHAR(64), not Postgres's unbounded TEXT — a SHA-256 hex digest is
    -- always exactly 64 chars, and this column sits in a UNIQUE index below
    -- (MySQL/InnoDB needs a fixed indexable width for that).
    token_hash        VARCHAR(64),   -- SHA-256 hex, direct-websocket only
    credential_ref_id CHAR(36),          -- credential-broker-service CredentialMetadata.id, relay-websocket only
    created_at        TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_used_at      TIMESTAMP(6),
    revoked_at        TIMESTAMP(6),

    CONSTRAINT exactly_one_secret_ref CHECK (
        (token_hash IS NOT NULL AND credential_ref_id IS NULL) OR
        (token_hash IS NULL AND credential_ref_id IS NOT NULL)
    )
);

-- No partial-index WHERE clause needed here (unlike the other partial
-- indexes in this rollout): MySQL, like Postgres, treats every NULL in a
-- UNIQUE index as distinct from every other NULL, so rows with
-- token_hash IS NULL never collide with each other even without a WHERE
-- filter — the plain index already has identical semantics to the
-- Postgres partial one.
CREATE UNIQUE INDEX idx_agent_tokens_hash ON agent_tokens (token_hash);
-- MySQL/TiDB has no partial index — full index instead of Postgres's
-- `WHERE revoked_at IS NULL`.
CREATE INDEX idx_agent_tokens_dev_server_active ON agent_tokens (dev_server_id);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.
