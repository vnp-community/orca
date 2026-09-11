-- MySQL/TiDB variant of postgres/0010_workflow_outbox.up.sql — same shape
-- as usage-service's/issue-tracking-service's outbox_events mysql variant.
-- payload JSON instead of JSONB (MySQL has native JSON but no JSONB ->/@>
-- operators) — common/outbox.Relay only reads payload whole to publish it,
-- never queries by JSON operator, so nothing is lost here.
CREATE TABLE outbox_events (
    id            CHAR(36) NOT NULL PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
);

-- MySQL has no partial/filtered index — a plain composite index over
-- (created_at, published_at) serves the same "oldest unpublished first"
-- poll the Postgres partial index was for, just without the "small
-- regardless of published history size" property.
CREATE INDEX idx_workflow_outbox_events_unpublished ON outbox_events (created_at, published_at);
