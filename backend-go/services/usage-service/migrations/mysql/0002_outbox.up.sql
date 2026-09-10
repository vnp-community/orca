-- MySQL/TiDB variant: payload JSON thay JSONB (MySQL 5.7+/TiDB có JSON
-- native nhưng KHÔNG có toán tử ->/@> kiểu JSONB của Postgres — outbox
-- relay (common/outbox.Relay) chỉ đọc payload nguyên khối để publish,
-- không query theo JSON operator, nên không mất chức năng ở use case này).
CREATE TABLE outbox_events (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    occurred_at   TIMESTAMP(6) NOT NULL,
    version       INT NOT NULL,
    payload       JSON NOT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at  TIMESTAMP(6) NULL
);

CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at, published_at);
