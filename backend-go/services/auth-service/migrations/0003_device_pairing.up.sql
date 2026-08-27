CREATE TABLE auth.pairing_sessions (
    id                              TEXT PRIMARY KEY,   -- hash of the pairing token
    tenant_id                       UUID NOT NULL,
    user_id                         UUID NOT NULL REFERENCES auth.users(id),
    desktop_public_key              BYTEA NOT NULL,
    desktop_private_key_ciphertext  BYTEA NOT NULL,
    vault_key_ref                   TEXT NOT NULL,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at                      TIMESTAMPTZ NOT NULL,  -- BR-MB-01
    consumed_at                     TIMESTAMPTZ             -- BR-MB-02
);
CREATE INDEX idx_pairing_sessions_expires_at ON auth.pairing_sessions(expires_at); -- reaper job, mirrors sessions/refresh_tokens

CREATE TABLE auth.paired_devices (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                 UUID NOT NULL,
    user_id                   UUID NOT NULL REFERENCES auth.users(id),
    device_label              TEXT,
    shared_secret_ciphertext  BYTEA NOT NULL,
    vault_key_ref             TEXT NOT NULL,
    status                    TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
    paired_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at              TIMESTAMPTZ,
    revoked_at                TIMESTAMPTZ
);
CREATE INDEX idx_paired_devices_user_active ON auth.paired_devices(tenant_id, user_id) WHERE status = 'active'; -- backs BR-MB-03's count check
