-- MySQL/TiDB translation of postgres/0001_init.up.sql — dialect-safe per
-- BE-DB-SOL-001 §5. No `CREATE SCHEMA` / `project.` prefix: this service's
-- MySQL DSN points directly at a database named `project` (the database
-- itself is the isolation unit MySQL uses in place of a Postgres schema).
CREATE TABLE projects (
    id              CHAR(36) PRIMARY KEY,
    tenant_id       CHAR(36) NOT NULL,
    name            TEXT NOT NULL,
    dev_server_id   CHAR(36),
    created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
CREATE INDEX idx_projects_tenant ON projects (tenant_id);
CREATE INDEX idx_projects_dev_server ON projects (dev_server_id);

-- Row-Level Security dropped — MySQL/TiDB has no RLS equivalent. Per
-- BE-DB-SOL-001 §4, Postgres RLS here was never actually enforced either
-- (no Go code in backend-go issues `SET LOCAL app.tenant_id`), so dropping
-- it removes no real protection, only a backstop that never activated.
-- Application-layer tenant_id filtering (internal/adapter/*/repository.go)
-- is, and always was, the real enforcement — see TestRepository_*_DoesNotLeakAcrossTenants.

CREATE TABLE project_members (
    project_id  CHAR(36) NOT NULL,
    user_id     CHAR(36) NOT NULL,
    role        VARCHAR(16) NOT NULL CHECK (role IN ('member', 'owner')),
    added_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (project_id, user_id),
    CONSTRAINT fk_project_members_project FOREIGN KEY (project_id) REFERENCES projects (id) ON DELETE CASCADE
);
CREATE INDEX idx_project_members_user ON project_members (user_id);
