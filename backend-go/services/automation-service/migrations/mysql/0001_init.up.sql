-- automation-service owns this database exclusively — no other service
-- reads or writes these tables. MySQL/TiDB variant: no CREATE SCHEMA (a
-- MySQL database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named `automation`,
-- mirroring the Postgres variant's `automation` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md's
-- usage-service precedent).
--
-- id has no server-side default here (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): internal/usecase/create_automation.go and
-- run_now.go always generate the id in Go (uuid.NewString()) before calling
-- the repository, so the Postgres default was already dead code from the
-- application's point of view — same finding as BE-DB-SOL-001 §1
-- (usage-service) and BE-DB-SOL-005 §1 (annotation-service). No functional
-- loss.
CREATE TABLE automations (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    name                TEXT NOT NULL,
    rrule               TEXT NOT NULL,
    dtstart             TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    step_config_json    TEXT NOT NULL,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_automations_tenant ON automations (tenant_id);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. See
-- TASK-BE-DB-003's finding (mirrored for this service, not re-audited from
-- scratch — same codebase-wide fact): no Go code anywhere calls
-- SET LOCAL app.tenant_id, so the Postgres RLS policy below never actually
-- activated either. This migration doesn't regress anything, it just stops
-- declaring a backstop that never ran:
--
--   ALTER TABLE automation.automations ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON automation.automations
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Run bookkeeping. status/step_type/output_json/error_message record the
-- REAL outcome workflow-service reported back.
--
-- request_id is bounded to VARCHAR(255) here (Postgres: unbounded TEXT) —
-- the UNIQUE KEY below needs an indexable column, and InnoDB cannot index
-- unbounded TEXT without an explicit prefix length; VARCHAR(255) is ample
-- for a caller-supplied idempotency key and matches usage-service's own
-- MySQL request_id bound (VARCHAR(128), see that service's
-- migrations/mysql/0001_init.up.sql) rather than introducing a prefix
-- index here.
CREATE TABLE automation_runs (
    id                  CHAR(36) PRIMARY KEY,
    automation_id       CHAR(36) NOT NULL,
    tenant_id           CHAR(36) NOT NULL,
    request_id          VARCHAR(255) NOT NULL,
    status              TEXT NOT NULL,
    -- MySQL/InnoDB rejects a plain literal DEFAULT on TEXT/BLOB/JSON
    -- columns ("Error 1101") — wrapping the literal in parentheses turns
    -- it into an expression default, which MySQL 8.0.13+ does accept;
    -- confirmed against the real mysql:8 test image (see
    -- BE-DB-SOL-012 §Kết quả thực tế).
    step_type           TEXT NOT NULL DEFAULT (''),
    step_config_json    TEXT NOT NULL,
    output_json         TEXT,
    error_message       TEXT,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    started_at          TIMESTAMP(6) NULL,
    completed_at        TIMESTAMP(6) NULL,

    CONSTRAINT automation_runs_status_check CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    CONSTRAINT fk_automation_runs_automation FOREIGN KEY (automation_id)
        REFERENCES automations (id) ON DELETE CASCADE,

    -- Idempotency backstop, mirroring usage-service's (tenant_id,
    -- request_id) pattern — a retried or duplicate-ticked RunNow dispatch
    -- for the same occurrence must not create a second row.
    UNIQUE KEY uniq_tenant_request (tenant_id, request_id)
);

CREATE INDEX idx_automation_runs_automation ON automation_runs (automation_id, created_at DESC);

-- No RLS equivalent — see automations table's comment above; the same
-- policy existed on automation_runs in Postgres and never actually
-- activated there either:
--
--   ALTER TABLE automation.automation_runs ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON automation.automation_runs
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
