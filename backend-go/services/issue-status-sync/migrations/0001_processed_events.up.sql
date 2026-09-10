CREATE SCHEMA IF NOT EXISTS issuestatussync;

CREATE TABLE issuestatussync.processed_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Dedup cache only (08-inter-service-communication.md:42-45), not an audit
-- log — a short-TTL cleanup job (e.g. 7-day retention) should be added as a
-- follow-up, not required for this task.
