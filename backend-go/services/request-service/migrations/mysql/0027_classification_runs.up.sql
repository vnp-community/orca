CREATE TABLE classification_runs (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    trigger_name VARCHAR(64) NOT NULL,
    source_event_id CHAR(36) NULL,
    manual TINYINT(1) NOT NULL DEFAULT 0,
    actor_id CHAR(36) NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('running','succeeded','failed')),
    claims INT NOT NULL DEFAULT 1,
    lease_owner VARCHAR(255) NOT NULL DEFAULT '',
    lease_expires_at TIMESTAMP(6) NOT NULL,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    started_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    finished_at TIMESTAMP(6) NULL,
    -- 1 while running, NULL after: NULLs never collide, so the unique key allows one live run per request.
    active TINYINT NULL,
    CONSTRAINT fk_classification_runs_request FOREIGN KEY (request_id) REFERENCES requests(id),
    UNIQUE KEY classification_runs_one_running (tenant_id, request_id, active),
    UNIQUE KEY classification_runs_event (tenant_id, source_event_id),
    INDEX classification_runs_lease_scan (status, lease_expires_at)
) ENGINE=InnoDB;
