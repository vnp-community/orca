-- Outcome Review (CR-OR-022, CR-OR-012, CR-OR-013): what a person decided about a review, who approved it, how a regeneration is
-- tracked, and the drawing profile a tenant or project saved.

-- One decision in force for each (review, item). A changed mind is a new row; the old one is kept as history with active_key cleared.
CREATE TABLE outcome_review_decisions (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    review_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    item_key VARCHAR(64) NOT NULL,
    decision VARCHAR(20) NOT NULL CHECK (decision IN ('accept_exception','send_back','dismiss')),
    note TEXT NOT NULL DEFAULT (''),
    child_request_id CHAR(36) NULL,
    decided_by CHAR(36) NOT NULL,
    decided_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    active_key VARCHAR(200) NULL,
    UNIQUE KEY uq_outcome_review_decisions_active (active_key),
    INDEX idx_outcome_review_decisions_review (tenant_id, review_id, decided_at)
) ENGINE=InnoDB;

ALTER TABLE outcome_reviews
    ADD COLUMN approved_by CHAR(36) NULL,
    ADD COLUMN approved_at TIMESTAMP(6) NULL,
    ADD COLUMN approval_note TEXT NOT NULL DEFAULT ('');

-- reason is why a person regenerated (empty for a run the system started); client_key makes a repeated request one run.
ALTER TABLE outcome_review_runs
    ADD COLUMN reason VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN client_key VARCHAR(64) NULL,
    ADD COLUMN created_by CHAR(36) NULL,
    ADD UNIQUE KEY uq_outcome_review_runs_client (tenant_id, request_id, client_key),
    ADD INDEX idx_outcome_review_runs_reason (tenant_id, request_id, started_at);

-- project_key is the project id, or empty for the tenant layer.
CREATE TABLE review_drawing_profiles (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    project_key VARCHAR(36) NOT NULL DEFAULT '',
    spec JSON NOT NULL,
    spec_digest VARCHAR(64) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    updated_by CHAR(36) NOT NULL,
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_review_drawing_profiles_scope (tenant_id, project_key)
) ENGINE=InnoDB;
