CREATE TABLE approval_policies (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    project_id CHAR(36) NULL,
    subject_type VARCHAR(20) NOT NULL,
    request_type VARCHAR(20) NULL,
    size VARCHAR(2) NULL,
    urgency VARCHAR(20) NULL,
    approvers JSON NOT NULL,
    allow_requester_approve TINYINT(1) NOT NULL DEFAULT 0,
    due_after_seconds INT NULL,
    priority INT NOT NULL DEFAULT 0,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    version BIGINT NOT NULL DEFAULT 1,
    created_by VARCHAR(64) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX approval_policies_subject (tenant_id, subject_type, enabled),
    CONSTRAINT chk_pol_subject_type CHECK (subject_type IN ('request_type','solution','findings','answer','plan','phase','task_list','pre_deploy')),
    CONSTRAINT chk_pol_request_type CHECK (request_type IN ('task','feature','bug','incident','problem','change','epic','story','subtask','release','other') OR request_type IS NULL),
    CONSTRAINT chk_pol_size CHECK (size IN ('S','M','L') OR size IS NULL),
    CONSTRAINT chk_pol_urgency CHECK (urgency IN ('normal','urgent') OR urgency IS NULL)
) ENGINE=InnoDB;

CREATE TABLE approval_approvers (
    approval_id CHAR(36) NOT NULL,
    principal_kind VARCHAR(20) NOT NULL,
    principal_id VARCHAR(64) NOT NULL,
    tenant_id CHAR(36) NOT NULL,
    PRIMARY KEY (approval_id, principal_kind, principal_id),
    INDEX approval_approvers_principal (tenant_id, principal_kind, principal_id),
    CONSTRAINT fk_approval_approvers_approval FOREIGN KEY (approval_id) REFERENCES approvals(id),
    CONSTRAINT chk_appr_principal_kind CHECK (principal_kind IN ('user','team','role','reporter'))
) ENGINE=InnoDB;
