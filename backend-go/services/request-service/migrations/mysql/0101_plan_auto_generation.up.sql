-- CR-REQ-044: Orca starts the Plan of a Request itself once its Analysis is approved. One job per cause keeps the state
-- (attempts, schedule, outcome) so it survives a restart and the Plan tab can show it after a reload.
CREATE TABLE plan_auto_jobs (
    tenant_id VARCHAR(36) NOT NULL,
    request_id VARCHAR(36) NOT NULL,
    anchor VARCHAR(128) NOT NULL,
    origin VARCHAR(10) NOT NULL,
    project_id VARCHAR(36) NOT NULL,
    state VARCHAR(16) NOT NULL,
    attempt INT NOT NULL DEFAULT 1,
    max_attempts INT NOT NULL DEFAULT 1,
    run_id VARCHAR(36) NULL,
    reason_code VARCHAR(128) NULL,
    commit_attempts INT NOT NULL DEFAULT 0,
    next_check_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    lease_owner VARCHAR(128) NULL,
    lease_expires_at TIMESTAMP(6) NULL,
    requested_by VARCHAR(36) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    -- NULL once the job is over, so the unique key below only constrains active jobs.
    active_key VARCHAR(36) GENERATED ALWAYS AS (IF(state IN ('pending','running','committing'), request_id, NULL)) STORED,
    PRIMARY KEY (tenant_id, request_id, anchor),
    CONSTRAINT fk_plan_auto_jobs_request FOREIGN KEY (request_id) REFERENCES requests(id),
    CONSTRAINT plan_auto_jobs_origin CHECK (origin IN ('event','retry')),
    CONSTRAINT plan_auto_jobs_state CHECK (state IN ('pending','running','committing','done','gave_up','skipped','superseded')),
    UNIQUE KEY plan_auto_jobs_one_active (tenant_id, active_key),
    INDEX plan_auto_jobs_due (state, next_check_at),
    INDEX plan_auto_jobs_by_request (tenant_id, request_id, created_at)
);

-- On unless a project turns it off; a project that never saved settings has no row and reads as on.
ALTER TABLE project_engine_settings
    ADD COLUMN plan_auto_generate TINYINT(1) NOT NULL DEFAULT 1;
