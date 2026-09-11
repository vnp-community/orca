-- One row per (notification, recipient) — NOT one row with an array of
-- recipients — so marking user A's copy read never touches user B's row,
-- same scoping unit push_subscriptions already uses (tenant_id, user_id).
-- PRIMARY KEY is (id, recipient_user_id), NOT id alone: Save (repository.go)
-- inserts the SAME domain.NotificationEvent.ID for every recipient row of a
-- multi-recipient event (id identifies "which translated event", not "which
-- row") — a lone `id UUID PRIMARY KEY` would collide on the 2nd+ recipient.
-- MarkAsRead already scopes every lookup by recipient_user_id too, so this
-- composite key doesn't weaken row isolation.
CREATE TABLE notification.notification_events (
    id                 UUID NOT NULL,                -- same value as domain.NotificationEvent.ID (uuid.NewString() in HandleIncomingEvent) — shared across a multi-recipient event's rows
    tenant_id          UUID NOT NULL,
    recipient_user_id  UUID NOT NULL,
    source_event_id    TEXT NOT NULL,
    source_subject     TEXT NOT NULL,
    type               TEXT NOT NULL,
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    deep_link          TEXT,
    severity           TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
    is_read            BOOLEAN NOT NULL DEFAULT false,
    read_at            TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, recipient_user_id)
);

-- Covers ListNotifications(unread_only=true) and GetUnreadCount's hot path:
-- filter by tenant+recipient(+is_read), order by created_at DESC.
CREATE INDEX idx_notification_events_recipient_unread
    ON notification.notification_events (tenant_id, recipient_user_id, is_read, created_at DESC);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id + recipient_user_id filtering
-- (internal/adapter/postgres) is the primary enforcement; this is the
-- secondary backstop, same pattern as push_subscriptions/vapid_key_metadata.
ALTER TABLE notification.notification_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.notification_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
