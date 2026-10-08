-- MySQL/TiDB variant of postgres/0002_request_sync_state (no schema prefix).
CREATE TABLE request_sync_state (
    tenant_id    VARCHAR(255) NOT NULL,
    request_id   VARCHAR(255) NOT NULL,
    last_version BIGINT       NOT NULL,
    last_target  VARCHAR(64)  NOT NULL,
    updated_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, request_id)
);
-- TODO: no retention yet (SOL-024 open question 5).
