-- One row per (notification, recipient) — NOT one row with an array of
-- recipients — so marking user A's copy read never touches user B's row,
-- same scoping unit push_subscriptions already uses (tenant_id, user_id).
-- PRIMARY KEY is (id, recipient_user_id), NOT id alone: SaveNotificationEvent
-- (internal/adapter/mysql/repository.go, mirroring the Postgres adapter)
-- inserts the SAME domain.NotificationEvent.ID for every recipient row of a
-- multi-recipient event (id identifies "which translated event", not
-- "which row") — a lone `id CHAR(36) PRIMARY KEY` would collide on the
-- 2nd+ recipient. MarkAsRead already scopes every lookup by
-- recipient_user_id too, so this composite key doesn't weaken row
-- isolation. UUID columns -> CHAR(36): id is always
-- event.ID == uuid.NewString() from Go (HandleIncomingEvent), never a
-- DB-side default.
CREATE TABLE notification_events (
    id                 CHAR(36) NOT NULL,
    tenant_id          CHAR(36) NOT NULL,
    recipient_user_id  CHAR(36) NOT NULL,
    source_event_id    TEXT NOT NULL,
    source_subject     TEXT NOT NULL,
    type               VARCHAR(128) NOT NULL,
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    deep_link          TEXT,
    severity           VARCHAR(16) NOT NULL,
    is_read            BOOLEAN NOT NULL DEFAULT false,
    read_at            TIMESTAMP(6) NULL,
    created_at         TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id, recipient_user_id),

    CONSTRAINT notification_events_severity_check CHECK (severity IN ('info','warning','critical'))
);

-- Covers ListByRecipient(unreadOnly=true) and CountUnread's hot path:
-- filter by tenant+recipient(+is_read), order by created_at DESC.
CREATE INDEX idx_notification_events_recipient_unread
    ON notification_events (tenant_id, recipient_user_id, is_read, created_at DESC);

-- No Row-Level Security equivalent in MySQL/TiDB — see 0001_init.up.sql's
-- comment; application-layer tenant_id + recipient_user_id filtering
-- (internal/adapter/mysql) is the only enforcement here too.
