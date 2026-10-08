-- MySQL translation of postgres/0021_task_execution_records.up.sql. MEDIUMTEXT so a 16 KB tail
-- of multi-byte text is never cut by the column limit.
CREATE TABLE task_execution_records (
    id                CHAR(36) NOT NULL PRIMARY KEY,
    tenant_id         CHAR(36) NOT NULL,
    task_id           CHAR(36) NOT NULL,
    execution_link_id CHAR(36) NULL,
    attempt           INT NOT NULL DEFAULT 1,
    spec_digest       CHAR(64) NOT NULL DEFAULT '',
    packet_digest     CHAR(64) NOT NULL DEFAULT '',
    template_version  VARCHAR(16) NOT NULL DEFAULT '',
    parse_status      VARCHAR(10) NOT NULL,
    failure_class     VARCHAR(16) NULL,
    result            JSON NULL,
    changes           JSON NULL,
    stdout_tail       MEDIUMTEXT NOT NULL,
    created_at        TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT task_execution_records_parse_status_check CHECK (parse_status IN ('ok','missing','invalid')),
    CONSTRAINT task_execution_records_failure_class_check CHECK (failure_class IS NULL OR failure_class IN ('retryable','needs_info','spec_defect','env_defect','agent_defect')),
    CONSTRAINT fk_task_execution_records_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    KEY idx_task_execution_records_task (tenant_id, task_id, created_at DESC)
) ENGINE=InnoDB;
