-- Outcome Review (CR-OR-001): one row per (request, phase, head). model is the whole review as JSON; the other columns are what the
-- service looks reviews up by. Rows are never deleted by the service; an old review is marked superseded.
CREATE TABLE outcome_reviews (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    phase VARCHAR(12) NOT NULL CHECK (phase IN ('design','delivered')),
    head_key VARCHAR(64) NOT NULL,
    head_commit VARCHAR(64) NOT NULL DEFAULT '',
    base_commit VARCHAR(64) NOT NULL DEFAULT '',
    inputs_digest VARCHAR(64) NOT NULL,
    state VARCHAR(12) NOT NULL CHECK (state IN ('draft','ready','stale','approved','superseded')),
    model JSON NOT NULL,
    kg_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    design_digest VARCHAR(64) NOT NULL DEFAULT '',
    context_digest VARCHAR(64) NOT NULL DEFAULT '',
    created_by CHAR(36) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    version BIGINT NOT NULL DEFAULT 1,
    UNIQUE KEY uq_outcome_reviews_key (tenant_id, request_id, phase, head_key),
    INDEX idx_outcome_reviews_request (tenant_id, request_id, created_at DESC, id DESC)
) ENGINE=InnoDB;

-- active_key is set only while the run is queued or running (NULLs do not collide in a unique key), so a second active run of the
-- same kind for a Request cannot be inserted; it is cleared when the run ends.
CREATE TABLE outcome_review_runs (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    review_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('context','draw')),
    status VARCHAR(12) NOT NULL CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
    scope VARCHAR(64) NOT NULL DEFAULT '',
    agent_run_ref VARCHAR(255) NOT NULL DEFAULT '',
    cost_estimate VARCHAR(64) NOT NULL DEFAULT '',
    cost_actual VARCHAR(64) NOT NULL DEFAULT '',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    active_key VARCHAR(120) NULL,
    started_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    finished_at TIMESTAMP(6) NULL,
    UNIQUE KEY uq_outcome_review_runs_active (active_key),
    INDEX idx_outcome_review_runs_review (tenant_id, review_id, started_at DESC),
    INDEX idx_outcome_review_runs_request (tenant_id, request_id, kind, started_at DESC)
) ENGINE=InnoDB;
