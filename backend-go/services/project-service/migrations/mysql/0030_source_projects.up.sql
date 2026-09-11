CREATE TABLE source_projects (
    id                    CHAR(36) PRIMARY KEY,
    container_project_id  CHAR(36) NOT NULL,
    source_project_id     CHAR(36) NOT NULL,
    linked_by             CHAR(36) NOT NULL,
    linked_at             DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CHECK (container_project_id != source_project_id),
    UNIQUE (container_project_id, source_project_id),
    CONSTRAINT fk_source_projects_container FOREIGN KEY (container_project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_source_projects_source FOREIGN KEY (source_project_id) REFERENCES projects (id) ON DELETE CASCADE
);
CREATE INDEX idx_source_projects_container ON source_projects (container_project_id);

-- RLS dropped — see 0001_init.up.sql's comment.
