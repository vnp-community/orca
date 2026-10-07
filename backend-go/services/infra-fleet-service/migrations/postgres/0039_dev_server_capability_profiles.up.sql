CREATE TABLE infra.dev_server_capability_profiles (
    dev_server_id       UUID PRIMARY KEY REFERENCES infra.dev_servers(id) ON DELETE CASCADE,
    tenant_id           UUID NOT NULL,
    source              TEXT NOT NULL CHECK (source IN ('probe', 'handshake_only')),
    agent_build_version TEXT NOT NULL DEFAULT '',
    protocol_version    INT NOT NULL DEFAULT 1,
    features            JSONB NOT NULL DEFAULT '[]'::jsonb,
    profile             JSONB NOT NULL DEFAULT '{}'::jsonb,
    fingerprint         CHAR(64) NOT NULL,
    probed_at           TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dev_server_capability_profiles_tenant ON infra.dev_server_capability_profiles (tenant_id, probed_at DESC);

ALTER TABLE infra.dev_server_capability_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.dev_server_capability_profiles
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
