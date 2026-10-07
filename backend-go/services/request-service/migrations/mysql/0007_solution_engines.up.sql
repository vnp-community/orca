CREATE TABLE project_engine_settings (
    tenant_id VARCHAR(36) NOT NULL,
    project_id VARCHAR(36) NOT NULL,
    solution_engine ENUM('native','openspec') NOT NULL,
    openspec_min_version VARCHAR(64) NULL,
    updated_by VARCHAR(36) NOT NULL,
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    version BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, project_id)
);

CREATE TABLE openspec_changes (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    request_id VARCHAR(36) NOT NULL,
    change_id VARCHAR(80) NOT NULL,
    branch VARCHAR(255) NULL,
    status ENUM('preparing','ready','archived','abandoned') NOT NULL,
    tasks_sync_state ENUM('in_sync','pending','failed') NOT NULL,
    tasks_sync_digest VARCHAR(128) NULL,
    tasks_sync_error TEXT NULL,
    tasks_sync_next_retry_at TIMESTAMP(6) NULL,
    commit_sha VARCHAR(40) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_openspec_changes_request FOREIGN KEY (request_id) REFERENCES requests(id),
    UNIQUE KEY openspec_changes_one_request (tenant_id, request_id),
    INDEX openspec_changes_sync (tenant_id, status, tasks_sync_state)
);

ALTER TABLE requests ADD COLUMN solution_engine ENUM('native','openspec') NULL;
ALTER TABLE analysis_runs ADD COLUMN engine VARCHAR(32) NOT NULL DEFAULT 'native';
ALTER TABLE analysis_runs DROP CHECK analysis_runs_chk_1; -- Assuming the name, MySQL >= 8.0.16
ALTER TABLE analysis_runs ADD CONSTRAINT analysis_runs_mode_check CHECK (mode IN ('complete','agent_readonly','agent_proposal'));
