-- Transactional outbox: RotateCredential writes its event here in the same
-- transaction as the status change; common/outbox.Relay publishes it to NATS.
-- Payload never carries secret material (ids, category, timestamp only).
CREATE TABLE credential.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_credential_outbox_events_unpublished
    ON credential.outbox_events (created_at)
    WHERE published_at IS NULL;
