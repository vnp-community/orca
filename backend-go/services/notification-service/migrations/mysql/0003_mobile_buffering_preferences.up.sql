-- BL-MB-02 (SOL-MB-02): offline push buffering + per-event-type
-- preferences for the mobile companion app. MySQL/TiDB variant — no
-- CREATE SCHEMA (see 0001_init.up.sql's comment).

-- BR-MB-07: offline buffering, max 50 per (tenant,user,subscription).
-- Postgres's `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` is NOT
-- replicated here as a DB-side default: internal/adapter/postgres.
-- BufferedNotificationStore.Enqueue's INSERT never supplies an id column,
-- relying entirely on that server-side default — the one table in this
-- service where the id is NOT already generated in Go. Rather than lean on
-- MySQL's version-gated/non-deterministic UUID() default-expression
-- support, internal/adapter/mysql.BufferedNotificationStore.Enqueue
-- generates the id in Go (uuid.NewString(), same as every other table's
-- id), matching this service's dominant convention instead of introducing
-- a second one.
CREATE TABLE buffered_notifications (
    id                       CHAR(36) PRIMARY KEY,
    tenant_id                CHAR(36) NOT NULL,
    user_id                  CHAR(36) NOT NULL,
    subscription_id          CHAR(36) NOT NULL,
    -- JSONB -> JSON: MySQL has no JSONB type; JSON is its closest
    -- equivalent (stored as normalized text, no ->/@> operators). Neither
    -- operator is used against this column anywhere in this service (only
    -- decoded application-side via encoding/json in ListPending), so
    -- nothing here relies on JSONB-specific behavior.
    notification_event_json  JSON NOT NULL,
    buffered_at              TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    delivered_at             TIMESTAMP(6) NULL,

    CONSTRAINT fk_buffered_notifications_subscription
        FOREIGN KEY (subscription_id) REFERENCES push_subscriptions(id) ON DELETE CASCADE
);
-- Postgres's two indexes are *partial* (`WHERE delivered_at IS NULL`) —
-- MySQL has no WHERE clause on CREATE INDEX. Unlike idx_vapid_key_active
-- (0001_init), neither index here backs a uniqueness guarantee, only a
-- query-plan optimization, so the safe MySQL substitute is simply the same
-- columns without the predicate: still correct (ListPending/Enqueue's
-- eviction query still add `AND delivered_at IS NULL` in the WHERE
-- clause), just indexing some already-delivered rows too rather than only
-- the pending ones.
CREATE INDEX idx_buffered_notifications_pending ON buffered_notifications(subscription_id, buffered_at);
CREATE INDEX idx_buffered_notifications_user_pending ON buffered_notifications(tenant_id, user_id, buffered_at);

-- No Row-Level Security equivalent in MySQL/TiDB — see 0001_init.up.sql's
-- comment; application-layer tenant_id scoping is the only enforcement
-- here too.

-- BR-MB-08: per-event-type settings. Amendment to notification-service.md
-- §2's stated non-goal ("per-user notification preferences... out of
-- scope") — flagged here for reconciliation, not silently overridden: this
-- table exists because BL-MB-02's BR-MB-08 requires it for the mobile
-- companion app.
CREATE TABLE notification_preferences (
    tenant_id   CHAR(36) NOT NULL,
    user_id     CHAR(36) NOT NULL,
    event_type  VARCHAR(128) NOT NULL,
    channel     VARCHAR(16) NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    updated_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, user_id, event_type, channel),

    CONSTRAINT notification_preferences_channel_check CHECK (channel IN ('ws','web','ios','android'))
);
-- Absence of a row == enabled (default-on); this table only carries
-- explicit opt-outs, never a full cross-product seed per user.
