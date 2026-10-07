-- MySQL older than 8.0.16 will silently ignore CHECK constraints.
-- Application must enforce them in the domain logic.

CREATE TABLE request_counters (
    tenant_id CHAR(36) PRIMARY KEY,
    next_number BIGINT NOT NULL
) ENGINE=InnoDB;

CREATE TABLE requests (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    project_id CHAR(36),
    number BIGINT NOT NULL,
    title VARCHAR(500) NOT NULL,
    body TEXT NOT NULL,
    source_provider VARCHAR(20) NOT NULL DEFAULT '',
    source_ref VARCHAR(255) NOT NULL DEFAULT '',
    source_url TEXT NOT NULL,
    source_site VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(20) CHECK (type IN ('change_request','bug','hotfix','task','spike','question','refactor','security','performance','docs','ops_request')),
    type_source VARCHAR(20) CHECK (type_source IN ('ai','human')),
    size VARCHAR(40) CHECK (size IN ('S','M','L')),
    urgency VARCHAR(40) NOT NULL DEFAULT 'normal' CHECK (urgency IN ('normal','urgent')),
    confidence DECIMAL(4,3) CHECK (confidence >= 0 AND confidence <= 1),
    classification_reason TEXT NOT NULL,
    status VARCHAR(40) NOT NULL DEFAULT 'new' CHECK (status IN ('new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning','awaiting_plan_approval','executing','completed','request_backlog','cancelled')),
    returned_from_stage VARCHAR(40) CHECK (returned_from_stage IN ('classification','analysis','plan','phase','task')),
    return_reason TEXT NOT NULL,
    plan_task_id CHAR(36),
    reporter_id CHAR(36) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT requests_backlog_stage CHECK ((status = 'request_backlog') = (returned_from_stage IS NOT NULL)),
    UNIQUE (tenant_id, number),
    INDEX idx_requests_status_updated (tenant_id, status, updated_at DESC),
    INDEX idx_requests_project_status (tenant_id, project_id, status),
    INDEX idx_requests_plan_task (tenant_id, plan_task_id),
    INDEX idx_requests_source (tenant_id, source_provider, source_site, source_ref),
    INDEX idx_requests_pagination (tenant_id, created_at DESC, id DESC)
) ENGINE=InnoDB;

CREATE TABLE request_type_history (
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    from_type VARCHAR(20),
    to_type VARCHAR(20) NOT NULL,
    actor_id CHAR(36) NOT NULL,
    actor_kind VARCHAR(40) CHECK (actor_kind IN ('user','agent','system')),
    INDEX idx_request_type_history_at (request_id, at)
) ENGINE=InnoDB;

CREATE TABLE solutions (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    options JSON NOT NULL,
    chosen_option INT,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    INDEX idx_solutions_created (tenant_id, request_id, created_at)
) ENGINE=InnoDB;

CREATE TABLE request_links (
    tenant_id CHAR(36) NOT NULL,
    parent_request_id CHAR(36) NOT NULL,
    child_request_id CHAR(36) NOT NULL,
    reason VARCHAR(40) CHECK (reason IN ('relates_to', 'blocks', 'is_blocked_by', 'duplicates')),
    PRIMARY KEY (tenant_id, parent_request_id, child_request_id),
    CHECK (parent_request_id <> child_request_id),
    INDEX idx_request_links_child (tenant_id, child_request_id)
) ENGINE=InnoDB;

CREATE TABLE request_idempotency (
    tenant_id CHAR(36) NOT NULL,
    source_provider VARCHAR(20) NOT NULL,
    source_site VARCHAR(255) NOT NULL DEFAULT '',
    source_ref VARCHAR(255) NOT NULL CHECK (source_ref <> ''),
    request_id CHAR(36) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (tenant_id, source_provider, source_site, source_ref)
) ENGINE=InnoDB;
