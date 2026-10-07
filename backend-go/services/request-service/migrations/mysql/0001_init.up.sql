-- No schema or RLS in MySQL. Tenant isolation must be explicitly included in all queries.

CREATE TABLE outbox_events (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    subject TEXT NOT NULL,
    occurred_at TIMESTAMP(6) NOT NULL,
    version INT NOT NULL,
    payload JSON NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    published_at TIMESTAMP(6) NULL,
    seq BIGINT NOT NULL AUTO_INCREMENT,
    UNIQUE KEY uq_outbox_seq (seq),
    INDEX idx_outbox_unpublished (published_at, created_at, seq)
) ENGINE=InnoDB;

CREATE TABLE processed_events (
    tenant_id CHAR(36) NOT NULL,
    event_id CHAR(36) NOT NULL,
    subject TEXT NOT NULL,
    processed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, event_id),
    INDEX idx_processed_events_at (processed_at)
) ENGINE=InnoDB;
