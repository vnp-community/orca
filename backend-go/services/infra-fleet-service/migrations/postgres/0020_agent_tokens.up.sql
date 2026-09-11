-- Persistent, named, per-DevServer agent tokens (BL-AWS-03). Coexists with
-- (does not replace) the ephemeral bootstrap Registry/TokenIssuer in
-- adapter/agentwsserver — see usecase/create_agent_token.go's doc comment
-- for how the two are reconciled at handshake time.
CREATE TABLE infra.agent_tokens (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    dev_server_id     UUID NOT NULL REFERENCES infra.dev_servers(id),
    name              TEXT NOT NULL,
    -- Exactly one of token_hash / credential_ref_id is set, depending on
    -- the owning dev_server's connection_mode — see SOL-AWS-01 for why
    -- relay-websocket's row can't be a bare hash (Orca must itself present
    -- the plaintext outbound, so that case's secret lives in
    -- credential-broker-service/Vault, referenced here by id only).
    token_hash        TEXT,          -- SHA-256 hex, direct-websocket only
    credential_ref_id UUID,          -- credential-broker-service CredentialMetadata.id, relay-websocket only
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at      TIMESTAMPTZ,
    revoked_at        TIMESTAMPTZ,

    CONSTRAINT exactly_one_secret_ref CHECK (
        (token_hash IS NOT NULL AND credential_ref_id IS NULL) OR
        (token_hash IS NULL AND credential_ref_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_agent_tokens_hash ON infra.agent_tokens (token_hash)
    WHERE token_hash IS NOT NULL;
CREATE INDEX idx_agent_tokens_dev_server_active ON infra.agent_tokens (dev_server_id)
    WHERE revoked_at IS NULL;

ALTER TABLE infra.agent_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.agent_tokens
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
