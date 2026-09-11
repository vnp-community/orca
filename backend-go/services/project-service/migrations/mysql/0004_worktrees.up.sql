CREATE TABLE worktrees (
    id         CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    repo_id    CHAR(36) NOT NULL,
    path       TEXT NOT NULL,
    branch     TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_worktrees_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_worktrees_repo FOREIGN KEY (repo_id) REFERENCES repos (id) ON DELETE CASCADE
);
CREATE INDEX idx_worktrees_project ON worktrees (project_id);
CREATE INDEX idx_worktrees_repo ON worktrees (repo_id);

-- RLS dropped — see 0001_init.up.sql's comment.
