-- MySQL/TiDB variant of postgres/0014_step_approvals.up.sql (no partial index: the pending listing filters on status).
CREATE TABLE step_approvals (
    id            CHAR(36) NOT NULL PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    execution_id  CHAR(36) NOT NULL,
    step_id       VARCHAR(255) NOT NULL,
    project_id    VARCHAR(64) NOT NULL DEFAULT '',
    kind          VARCHAR(64) NOT NULL,
    subject       JSON NOT NULL,
    decision      JSON NULL,
    decision_hash VARCHAR(128) NOT NULL,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'superseded')),
    resolved_by   VARCHAR(64) NULL,
    rationale     TEXT NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    resolved_at   TIMESTAMP(6) NULL,
    submitted_at  TIMESTAMP(6) NULL,
    CONSTRAINT fk_workflow_step_approvals_execution FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE,
    UNIQUE (execution_id, step_id, decision_hash)
);

CREATE INDEX idx_workflow_step_approvals_pending ON step_approvals (tenant_id, status, project_id);
CREATE INDEX idx_workflow_step_approvals_step ON step_approvals (execution_id, step_id);
