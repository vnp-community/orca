ALTER TABLE requests ADD COLUMN returned_category VARCHAR(30) NULL,
    ADD CONSTRAINT requests_returned_category_values CHECK (returned_category IN ('missing_info','infeasible','blocked_dependency','rejected','other'));

UPDATE requests SET returned_category = 'other' WHERE status = 'request_backlog';

ALTER TABLE requests ADD CONSTRAINT requests_backlog_category
    CHECK ((status = 'request_backlog') = (returned_category IS NOT NULL));

CREATE TABLE request_return_history (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    action VARCHAR(20) NOT NULL CHECK (action IN ('returned','reopened','cancelled')),
    stage VARCHAR(40) NULL CHECK (stage IN ('classification','analysis','plan','phase','task')),
    category VARCHAR(30) NULL CHECK (category IN ('missing_info','infeasible','blocked_dependency','rejected','other')),
    reason TEXT NOT NULL,
    actor_id CHAR(36) NULL,
    actor_kind VARCHAR(20) NOT NULL CHECK (actor_kind IN ('user','agent','system')),
    at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    INDEX idx_request_return_history (tenant_id, request_id, at)
) ENGINE=InnoDB;
