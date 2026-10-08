CREATE TABLE dev_server_capability_profiles (
    dev_server_id       CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    source              VARCHAR(16) NOT NULL,
    agent_build_version VARCHAR(128) NOT NULL DEFAULT '',
    protocol_version    INT NOT NULL DEFAULT 1,
    features            JSON NOT NULL,
    profile             JSON NOT NULL,
    fingerprint         CHAR(64) NOT NULL,
    probed_at           TIMESTAMP(6) NOT NULL,
    updated_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT dev_server_capability_profiles_source_check CHECK (source IN ('probe', 'handshake_only')),
    CONSTRAINT fk_dev_server_capability_profiles_dev_server FOREIGN KEY (dev_server_id) REFERENCES dev_servers(id) ON DELETE CASCADE
);

CREATE INDEX idx_dev_server_capability_profiles_tenant ON dev_server_capability_profiles (tenant_id, probed_at DESC);
