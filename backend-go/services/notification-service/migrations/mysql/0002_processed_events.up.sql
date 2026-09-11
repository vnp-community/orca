-- Consumer-side dedup for JetStream's at-least-once delivery, per
-- notification-service.md §5/§8 and architecture/08-inter-service-communication.md's
-- idempotency rule. Short-retention operational table, not an audit log —
-- only needs to cover JetStream's realistic redelivery window (§8 suggests
-- a ~7 day pruning window), not forever. UUID event_id -> CHAR(36), same
-- "id generated in Go before the call" reasoning as 0001_init.
CREATE TABLE processed_events (
    event_id      CHAR(36) PRIMARY KEY,
    subject       TEXT NOT NULL,
    processed_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
CREATE INDEX idx_processed_events_processed_at ON processed_events(processed_at);
