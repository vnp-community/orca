-- Consumer-side dedup for JetStream's at-least-once delivery, per
-- notification-service.md §5/§8 and architecture/08-inter-service-communication.md's
-- idempotency rule. Short-retention operational table, not an audit log —
-- only needs to cover JetStream's realistic redelivery window (§8 suggests
-- a ~7 day pruning window), not forever.
CREATE TABLE notification.processed_events (
    event_id      UUID PRIMARY KEY,
    subject       TEXT NOT NULL,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_processed_events_processed_at ON notification.processed_events(processed_at);
