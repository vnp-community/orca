CREATE TABLE scm.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_scm_outbox_events_unpublished
    ON scm.outbox_events (created_at)
    WHERE published_at IS NULL;

ALTER TABLE scm.outbox_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON scm.outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
