ALTER TABLE solutions
    ADD COLUMN kind ENUM('solution','diagnosis','findings','answer') NOT NULL DEFAULT 'solution',
    ADD COLUMN status ENUM('draft','proposed','approved','rejected','superseded') NOT NULL DEFAULT 'draft',
    ADD COLUMN content_ref VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN generation_run_id CHAR(36) NULL,
    ADD INDEX idx_solutions_request_state (tenant_id, request_id, kind, status);

ALTER TABLE analysis_runs
    ADD COLUMN solution_id CHAR(36) NULL,
    ADD COLUMN project_id CHAR(36) NULL,
    ADD COLUMN actor_id CHAR(36) NULL,
    ADD COLUMN feedback TEXT NULL,
    ADD COLUMN enforcement ENUM('agent_enforced','prompt_only') NULL,
    ADD COLUMN repo_check ENUM('skipped','clean','modified') NULL,
    ADD INDEX analysis_runs_project_running (tenant_id, project_id, mode, status);

-- One row per project is the lock that serialises the agent_readonly concurrency check (MySQL has no partial index to count on).
CREATE TABLE analysis_project_gates (
    tenant_id CHAR(36) NOT NULL,
    project_id CHAR(36) NOT NULL,
    PRIMARY KEY (tenant_id, project_id)
) ENGINE=InnoDB;
