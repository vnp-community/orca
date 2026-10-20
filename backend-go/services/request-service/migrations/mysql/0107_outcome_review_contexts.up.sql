-- Outcome Review (CR-OR-002): what a context run produced, kept so the next review does not ask the agent again. kind=design is
-- the locked design (reused by the delivered phase and by later runs); kind=context is a whole checked result, keyed by its digest.
CREATE TABLE outcome_review_contexts (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    kind VARCHAR(12) NOT NULL CHECK (kind IN ('design','context')),
    digest VARCHAR(64) NOT NULL,
    prompt_version VARCHAR(80) NOT NULL DEFAULT '',
    payload JSON NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at TIMESTAMP(6) NOT NULL,
    UNIQUE KEY uq_outcome_review_contexts (tenant_id, request_id, kind, digest),
    INDEX idx_outcome_review_contexts_latest (tenant_id, request_id, kind, created_at DESC)
) ENGINE=InnoDB;
