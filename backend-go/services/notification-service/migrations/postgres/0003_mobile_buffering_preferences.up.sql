-- BL-MB-02 (SOL-MB-02): offline push buffering + per-event-type
-- preferences for the mobile companion app.

-- BR-MB-07: offline buffering, max 50 per (tenant,user,subscription).
CREATE TABLE notification.buffered_notifications (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID NOT NULL,
    user_id                  UUID NOT NULL,
    subscription_id          UUID NOT NULL REFERENCES notification.push_subscriptions(id) ON DELETE CASCADE,
    notification_event_json  JSONB NOT NULL,
    buffered_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at             TIMESTAMPTZ
);
CREATE INDEX idx_buffered_notifications_pending ON notification.buffered_notifications(subscription_id, buffered_at)
    WHERE delivered_at IS NULL;
CREATE INDEX idx_buffered_notifications_user_pending ON notification.buffered_notifications(tenant_id, user_id, buffered_at)
    WHERE delivered_at IS NULL;

ALTER TABLE notification.buffered_notifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.buffered_notifications
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- BR-MB-08: per-event-type settings. Amendment to notification-service.md
-- §2's stated non-goal ("per-user notification preferences... out of
-- scope") — flagged here for reconciliation, not silently overridden: this
-- table exists because BL-MB-02's BR-MB-08 requires it for the mobile
-- companion app.
CREATE TABLE notification.notification_preferences (
    tenant_id   UUID NOT NULL,
    user_id     UUID NOT NULL,
    event_type  TEXT NOT NULL,
    channel     TEXT NOT NULL CHECK (channel IN ('ws','web','ios','android')),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, event_type, channel)
);
-- Absence of a row == enabled (default-on); this table only carries
-- explicit opt-outs, never a full cross-product seed per user.

ALTER TABLE notification.notification_preferences ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notification.notification_preferences
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
