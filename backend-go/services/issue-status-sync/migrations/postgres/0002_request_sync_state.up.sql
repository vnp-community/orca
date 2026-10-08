-- Last Request status version already pushed to the tracker, per Request.
-- Guards against stale or reordered request events; processed_events only
-- dedups by event_id. No RLS: this service owns its database (as 0001).
CREATE TABLE issuestatussync.request_sync_state (
    tenant_id    TEXT        NOT NULL,
    request_id   TEXT        NOT NULL,
    last_version BIGINT      NOT NULL,
    last_target  TEXT        NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, request_id)
);
-- TODO: no retention yet (SOL-024 open question 5).
