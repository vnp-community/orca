CREATE TABLE project_host_setups (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    dev_server_id CHAR(36) NOT NULL,
    folder_path   TEXT NOT NULL,
    display_name  TEXT,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'validated', 'completed', 'failed')),
    project_id    CHAR(36),
    created_by    CHAR(36) NOT NULL,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_project_host_setups_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE SET NULL
);
CREATE INDEX idx_project_host_setups_tenant ON project_host_setups (tenant_id);

-- RLS dropped — see 0001_init.up.sql's comment.
