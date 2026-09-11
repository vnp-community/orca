-- MySQL/TiDB variant of postgres/0001_init.up.sql. No CREATE SCHEMA — a
-- MySQL database IS the schema-equivalent isolation unit; DATABASE_DSN for
-- this dialect points at a database already named `workflow`, mirroring
-- the Postgres variant's `workflow` schema (see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md, the
-- naming decision this service's rollout reuses unchanged).
--
-- id has no server-side default here (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): every INSERT in
-- internal/adapter/postgres/repository.go always supplies its own id
-- (Go's uuid.NewString(), confirmed by reading every usecase call site —
-- e.g. internal/usecase/create_template.go, execute.go), so the Postgres
-- default was already dead code from the application's point of view —
-- same finding BE-DB-SOL-001 §1 made for usage-service.sessions.id.
CREATE TABLE templates (
    id          CHAR(36) NOT NULL PRIMARY KEY,
    tenant_id   CHAR(36) NOT NULL,
    name        TEXT NOT NULL,
    dag_json    JSON NOT NULL,
    scope       VARCHAR(16) NOT NULL DEFAULT 'personal' CHECK (scope IN ('company', 'team', 'personal')),
    created_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

-- name is TEXT (unbounded) — InnoDB rejects a TEXT column in an index key
-- without an explicit prefix length (3072-byte key limit), unlike
-- Postgres's unbounded TEXT index. 255 chars covers any real template
-- name comfortably.
CREATE INDEX idx_workflow_templates_tenant ON templates (tenant_id, name(255));

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. Per
-- TASK-BE-DB-003's finding (codebase-wide fact, not re-audited per
-- service): no Go code anywhere calls SET LOCAL app.tenant_id, so the
-- Postgres RLS policy below never actually activated either — this
-- migration doesn't regress anything, it just stops declaring a backstop
-- that never ran:
--
--   ALTER TABLE workflow.templates ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON workflow.templates
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE executions (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    template_id    CHAR(36) NOT NULL,
    tenant_id      CHAR(36) NOT NULL,
    status         VARCHAR(16) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'cancelled')),
    root_trace_id  TEXT,
    paused_at      TIMESTAMP(6) NULL,
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_workflow_executions_template FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE
);

CREATE INDEX idx_workflow_executions_tenant_status ON executions (tenant_id, status, created_at DESC);
-- MySQL has no partial/filtered index — a plain index over (status) serves
-- the same boot-time recovery scan query, just without the "small
-- regardless of terminal-status row count" property the Postgres partial
-- index has.
CREATE INDEX idx_workflow_executions_resumable ON executions (status);

-- No RLS equivalent — see templates' comment above; same posture here.
