CREATE TABLE repos (
    id           CHAR(36) PRIMARY KEY,
    project_id   CHAR(36) NOT NULL,
    url          TEXT NOT NULL,
    display_name TEXT NOT NULL,
    position     INT  NOT NULL DEFAULT 0,
    created_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_repos_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);
CREATE INDEX idx_repos_project ON repos (project_id);

-- RLS dropped — see 0001_init.up.sql's comment.
