-- notification-service owns this database exclusively — no other service
-- reads or writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
-- No private_key column exists anywhere in this schema, ever — the VAPID
-- private key lives only in Vault Transit, mediated through
-- common/secrets.Client.TransitEncrypt (see notification-service.md §9).
CREATE SCHEMA IF NOT EXISTS notification;

CREATE TABLE notification.push_subscriptions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    user_id       UUID NOT NULL,
    channel       TEXT NOT NULL CHECK (channel IN ('web','ios','android')),
    endpoint      TEXT NOT NULL,       -- Web Push endpoint URL, or device token
    p256dh_key    TEXT,                -- Web Push subscription key (browser-issued, NOT VAPID)
    auth_key      TEXT,                -- Web Push subscription secret (browser-issued, NOT VAPID)
    device_label  TEXT,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','expired','revoked')),
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT web_keys_required CHECK (
        (channel <> 'web') OR (p256dh_key IS NOT NULL AND auth_key IS NOT NULL)
    )
);
CREATE UNIQUE INDEX idx_push_subscriptions_endpoint ON notification.push_subscriptions(endpoint);
CREATE INDEX idx_push_subscriptions_user ON notification.push_subscriptions(tenant_id, user_id, status);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE notification.push_subscriptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.push_subscriptions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Public half of the VAPID keypair only. Private half lives in Vault
-- Transit (§9) and is never a column here or in any backup of this table.
CREATE TABLE notification.vapid_key_metadata (
    key_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,       -- VAPID identity is per-tenant, not global
    public_key    TEXT NOT NULL,       -- base64url-encoded P-256 public key
    vault_key_ref TEXT NOT NULL,       -- Transit key name, e.g. "vapid-signing-<tenant_id>" — a pointer
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','rotating','revoked')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_vapid_key_active ON notification.vapid_key_metadata(tenant_id, status)
    WHERE status = 'active';

ALTER TABLE notification.vapid_key_metadata ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.vapid_key_metadata
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
