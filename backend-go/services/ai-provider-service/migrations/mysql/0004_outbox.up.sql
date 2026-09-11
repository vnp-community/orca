-- MySQL/TiDB variant: payload JSON thay JSONB — outbox relay
-- (common/outbox.Relay) chỉ đọc payload nguyên khối để publish, không
-- query theo JSON operator, nên không mất chức năng ở use case này (cùng
-- lý do usage-service's 0002_outbox.up.sql đã ghi).
CREATE TABLE outbox (
    id           CHAR(36) PRIMARY KEY,
    tenant_id    CHAR(36) NOT NULL,
    subject      VARCHAR(255) NOT NULL,
    occurred_at  TIMESTAMP(6) NOT NULL,
    version      INT NOT NULL,
    payload      JSON NOT NULL,
    created_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at TIMESTAMP(6) NULL
);

-- Partial index (`WHERE published_at IS NULL`) has no MySQL equivalent —
-- indexes the full (created_at, published_at) pair instead, same
-- translation usage-service's idx_outbox_events_unpublished already uses.
CREATE INDEX idx_ai_provider_outbox_unpublished ON outbox (created_at, published_at);
