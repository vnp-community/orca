-- Idempotency key of "start this Phase": pressing the button twice must not dispatch twice.
CREATE TABLE phase_starts (
    tenant_id CHAR(36) NOT NULL,
    phase_task_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    started_by CHAR(36) NOT NULL,
    started_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, phase_task_id),
    INDEX phase_starts_request (tenant_id, request_id)
) ENGINE=InnoDB;

-- One row per task event we acted on. event_id is unique so a redelivered event cannot count a failure twice.
CREATE TABLE task_run_outcomes (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    task_id CHAR(36) NOT NULL,
    container_id CHAR(36) NULL,
    outcome VARCHAR(12) NOT NULL CHECK (outcome IN ('started','succeeded','failed','cancelled','phase_done','plan_done')),
    cause VARCHAR(40) NOT NULL DEFAULT '',
    execution_link_id CHAR(36) NULL,
    error_message TEXT NOT NULL,
    event_id CHAR(36) NOT NULL,
    occurred_at TIMESTAMP(6) NOT NULL,
    -- 1 for phase_done/plan_done, NULL otherwise: NULLs never collide, so a container completes once.
    once TINYINT NULL CHECK (once IS NULL OR once = 1),
    UNIQUE KEY task_run_outcomes_event (tenant_id, event_id),
    UNIQUE KEY task_run_outcomes_once (tenant_id, task_id, once),
    INDEX idx_task_run_outcomes_request (tenant_id, request_id, occurred_at),
    INDEX idx_task_run_outcomes_task (tenant_id, task_id, outcome)
) ENGINE=InnoDB;

-- Cross-replica lease for the reconcile loop; last_run_at throttles re-evaluating a request that is simply waiting on a human.
CREATE TABLE execution_reconcile_state (
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    lease_owner VARCHAR(255) NOT NULL DEFAULT '',
    lease_until TIMESTAMP(6) NOT NULL DEFAULT '2000-01-01 00:00:01',
    last_run_at TIMESTAMP(6) NULL,
    PRIMARY KEY (tenant_id, request_id)
) ENGINE=InnoDB;

CREATE INDEX requests_executing_scan ON requests (status, updated_at);
