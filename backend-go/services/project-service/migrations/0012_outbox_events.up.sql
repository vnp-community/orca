CREATE TABLE project.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_project_outbox_events_unpublished
    ON project.outbox_events (created_at)
    WHERE published_at IS NULL;

ALTER TABLE project.outbox_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
