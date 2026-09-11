CREATE TABLE project_groups (
    id              CHAR(36) PRIMARY KEY,
    tenant_id       CHAR(36) NOT NULL,
    name            TEXT NOT NULL,
    parent_group_id CHAR(36),
    created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_project_groups_parent FOREIGN KEY (parent_group_id) REFERENCES project_groups (id) ON DELETE CASCADE
);
CREATE INDEX idx_project_groups_tenant ON project_groups (tenant_id);
CREATE INDEX idx_project_groups_parent ON project_groups (parent_group_id);

-- RLS dropped — see 0001_init.up.sql's comment.
