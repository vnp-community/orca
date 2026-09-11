-- notification-service owns this database exclusively — no other service
-- reads or writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
-- No private_key column exists anywhere in this schema, ever — the VAPID
-- private key lives only in Vault Transit, mediated through
-- common/secrets.Client.TransitEncrypt (see notification-service.md §9).
--
-- MySQL/TiDB variant: no CREATE SCHEMA (a MySQL database IS the
-- schema-equivalent isolation unit — this migration assumes DATABASE_DSN
-- already points at a database named `notification`, mirroring the
-- Postgres variant's `notification` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md §3's
-- naming convention). UUID columns become CHAR(36) (no
-- `DEFAULT gen_random_uuid()`): every caller (internal/usecase/subscribe.go
-- and its siblings) always generates the id in Go (uuid.NewString()) before
-- calling Save/GetPublicKey, so a DB-side default was never load-bearing.
CREATE TABLE push_subscriptions (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    user_id       CHAR(36) NOT NULL,
    channel       VARCHAR(16) NOT NULL,
    endpoint      TEXT NOT NULL,       -- Web Push endpoint URL, or device token
    p256dh_key    TEXT,                -- Web Push subscription key (browser-issued, NOT VAPID)
    auth_key      TEXT,                -- Web Push subscription secret (browser-issued, NOT VAPID)
    device_label  TEXT,
    status        VARCHAR(16) NOT NULL DEFAULT 'active',
    last_used_at  TIMESTAMP(6) NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    -- device_id added by 0004 in the Postgres timeline; folded in here since
    -- the MySQL rollout starts from a single combined baseline (no MySQL
    -- deployment existed before this rollout to migrate incrementally).
    device_id     CHAR(36) NULL,

    CONSTRAINT push_subscriptions_channel_check CHECK (channel IN ('web','ios','android')),
    CONSTRAINT push_subscriptions_status_check CHECK (status IN ('active','expired','revoked')),
    CONSTRAINT web_keys_required CHECK (
        (channel <> 'web') OR (p256dh_key IS NOT NULL AND auth_key IS NOT NULL)
    )
);
-- endpoint is TEXT, which MySQL cannot index at full length without a
-- prefix — 768 bytes covers every realistic Web Push endpoint URL / APNs
-- device token / FCM registration token with room to spare (all well under
-- 512 bytes in practice) while staying under InnoDB's utf8mb4 key-length
-- ceiling (3072 bytes / 4 bytes-per-char = 768 chars).
CREATE UNIQUE INDEX idx_push_subscriptions_endpoint ON push_subscriptions(endpoint(768));
CREATE INDEX idx_push_subscriptions_user ON push_subscriptions(tenant_id, user_id, status);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. Per
-- BE-DB-SOL-001 §4 (usage-service pilot finding, confirmed by grep: no
-- backend-go code ever calls `SET LOCAL app.tenant_id`, and no migration
-- uses `FORCE ROW LEVEL SECURITY`), the Postgres variant's RLS policy never
-- actually activated either — the pool's connecting role owns the tables
-- and bypasses RLS by default. This migration doesn't regress anything
-- real; it stops pretending a backstop exists that never ran. See this
-- rollout's tenant-isolation-without-RLS test for the proof.

-- Public half of the VAPID keypair only. Private half lives in Vault
-- Transit (§9) and is never a column here or in any backup of this table.
CREATE TABLE vapid_key_metadata (
    key_id        CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,   -- VAPID identity is per-tenant, not global
    public_key    TEXT NOT NULL,       -- base64url-encoded P-256 public key
    vault_key_ref VARCHAR(512) NOT NULL, -- Transit key name, e.g. "vapid-signing-<tenant_id>" — a pointer
    status        VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    revoked_at    TIMESTAMP(6) NULL,

    CONSTRAINT vapid_key_metadata_status_check CHECK (status IN ('active','rotating','revoked')),

    -- Postgres's idx_vapid_key_active is a *partial* unique index
    -- (`UNIQUE(tenant_id, status) WHERE status = 'active'`) — MySQL has no
    -- WHERE clause on CREATE INDEX. active_tenant_id is a stored generated
    -- column that collapses to NULL for every non-'active' row (MySQL, like
    -- Postgres, never enforces uniqueness across NULLs), so a plain UNIQUE
    -- index on it reproduces "at most one active key per tenant" exactly —
    -- the standard MySQL substitute for a partial unique index, not a
    -- weakened check.
    active_tenant_id CHAR(36) AS (CASE WHEN status = 'active' THEN tenant_id ELSE NULL END) STORED,
    UNIQUE KEY idx_vapid_key_active (active_tenant_id)
);
