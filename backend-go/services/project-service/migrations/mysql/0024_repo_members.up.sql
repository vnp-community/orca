CREATE TABLE repo_members (
    repo_id         CHAR(36) NOT NULL,
    user_id         CHAR(36) NOT NULL,
    functional_role VARCHAR(16) NOT NULL CHECK (functional_role IN ('developer', 'lead', 'admin')),
    added_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (repo_id, user_id),
    CONSTRAINT fk_repo_members_repo FOREIGN KEY (repo_id) REFERENCES repos (id) ON DELETE CASCADE
);
CREATE INDEX idx_repo_members_user ON repo_members (user_id);

-- RLS dropped — see 0001_init.up.sql's comment.
