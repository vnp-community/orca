-- MySQL/TiDB variant of postgres/0001_init.up.sql. No CREATE SCHEMA, no
-- pgcrypto extension — a MySQL database IS the schema-equivalent isolation
-- unit; DATABASE_DSN for this dialect points at a database already named
-- `orchestration`, mirroring the Postgres variant's `orchestration` schema
-- (see specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md,
-- the naming decision this service's rollout reuses unchanged).
--
-- `id ... DEFAULT gen_random_uuid()` in the Postgres schema is never relied
-- on at runtime: every INSERT in internal/adapter/postgres/repository.go
-- mints the id in Go first (`uuid.NewString()`) and always supplies it as
-- a column value (confirmed by reading Create/CreateDispatchContext/
-- CreateGate/CreateWithTasks — none omit `id` from its column list), so the
-- MySQL columns below are plain CHAR(36) with no server-side default.
CREATE TABLE coordinator_runs (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    origin_task_id      TEXT NOT NULL, -- logical FK -> task-service.tasks.id (different id space, §2.1) — no SQL FK across databases
    spec                JSON NOT NULL DEFAULT ('{}'),
    status              VARCHAR(16) NOT NULL DEFAULT 'idle' CHECK (status IN ('idle','running','completed','failed')),
    coordinator_handle  TEXT NOT NULL,
    poll_interval_ms    INT NOT NULL DEFAULT 2000,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    completed_at        TIMESTAMP(6) NULL
);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. Per
-- BE-DB-SOL-001 §4's finding (confirmed by grep: no `SET LOCAL
-- app.tenant_id` call exists anywhere in backend-go), the Postgres RLS
-- policy this table's variant carried never actually activated either —
-- this migration doesn't regress protection, it stops declaring a
-- backstop that was never real. Every table below that had
-- `ENABLE ROW LEVEL SECURITY` in the Postgres variant (coordinator_runs,
-- orchestration_tasks, dispatch_contexts, decision_gates, messages) drops
-- it here for the same reason.

-- orchestration_tasks is THIS service's own DAG-node id space — see
-- postgres/0001_init.up.sql's comment; unchanged reasoning here.
CREATE TABLE orchestration_tasks (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    coordinator_run_id  CHAR(36) NOT NULL,
    parent_id           CHAR(36) NULL,
    origin_task_id      TEXT, -- root row only; logical FK -> task-service.tasks.id
    task_title          TEXT NOT NULL,
    spec                JSON NOT NULL DEFAULT ('{}'),
    status              VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','ready','dispatched','completed','failed','blocked')),
    deps                JSON NOT NULL DEFAULT ('[]'), -- sibling ids, same coordinator_run_id only — drives promotion
    result              JSON NULL,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    completed_at        TIMESTAMP(6) NULL,

    CONSTRAINT fk_otasks_run FOREIGN KEY (coordinator_run_id) REFERENCES coordinator_runs (id) ON DELETE CASCADE,
    CONSTRAINT fk_otasks_parent FOREIGN KEY (parent_id) REFERENCES orchestration_tasks (id)
);
CREATE INDEX idx_otasks_run_status ON orchestration_tasks (coordinator_run_id, status);

CREATE TABLE dispatch_contexts (
    id                     CHAR(36) PRIMARY KEY,
    tenant_id              CHAR(36) NOT NULL,
    -- Nullable: see postgres/0001_init.up.sql's comment re: the generated
    -- CreateDispatchContextRequest proto not always carrying a task id yet.
    orchestration_task_id  CHAR(36) NULL,
    -- VARCHAR, not TEXT: handle is part of idx_dispatch_handle below, and
    -- InnoDB rejects TEXT/BLOB columns in a key without an explicit prefix
    -- length — same reasoning as issue-tracking-service's connections
    -- migration (BE-DB-SOL-004 §3).
    handle                 VARCHAR(255) NOT NULL,
    coordinator_run_id     CHAR(36) NOT NULL,
    status                 VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','dispatched','completed','failed','circuit_broken')),
    failure_count          INT NOT NULL DEFAULT 0,
    last_failure           TEXT,
    dispatched_at          TIMESTAMP(6) NULL,
    completed_at           TIMESTAMP(6) NULL,
    last_heartbeat_at      TIMESTAMP(6) NULL,
    created_at             TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_dispatch_task FOREIGN KEY (orchestration_task_id) REFERENCES orchestration_tasks (id) ON DELETE CASCADE,
    CONSTRAINT fk_dispatch_run FOREIGN KEY (coordinator_run_id) REFERENCES coordinator_runs (id) ON DELETE CASCADE
);
CREATE INDEX idx_dispatch_task ON dispatch_contexts (orchestration_task_id);
CREATE INDEX idx_dispatch_handle ON dispatch_contexts (tenant_id, handle);

CREATE TABLE decision_gates (
    id                     CHAR(36) PRIMARY KEY,
    tenant_id              CHAR(36) NOT NULL,
    orchestration_task_id  CHAR(36) NOT NULL,
    dispatch_context_id    CHAR(36) NULL,
    -- DEFAULT ('') not DEFAULT '': MySQL 8.0.13+ rejects a bare literal
    -- default on TEXT/BLOB/JSON columns (Error 1101) — only the
    -- parenthesized-expression default form is accepted, same reason the
    -- JSON columns above use DEFAULT ('{}')/DEFAULT ('[]').
    question               TEXT NOT NULL DEFAULT (''),
    options                JSON NOT NULL DEFAULT ('[]'),
    status                 VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','resolved','timeout')),
    resolution             TEXT,
    resolved_at            TIMESTAMP(6) NULL,
    created_at             TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_gates_task FOREIGN KEY (orchestration_task_id) REFERENCES orchestration_tasks (id) ON DELETE CASCADE,
    CONSTRAINT fk_gates_dispatch FOREIGN KEY (dispatch_context_id) REFERENCES dispatch_contexts (id)
);
-- Postgres's idx_gates_pending is a PARTIAL index (WHERE status='pending')
-- — MySQL has no partial/filtered index, so this is a plain composite
-- index over the same columns. Same "no longer small regardless of
-- resolved-gate history size" trade-off issue-tracking-service's
-- 0001_outbox.up.sql documents for its own unpublished-outbox index.
CREATE INDEX idx_gates_pending ON decision_gates (tenant_id, status);

-- messages: the coordinator's mailbox. `read` is a MySQL reserved word
-- (used in e.g. LOCK TABLES ... READ) and must be backtick-quoted wherever
-- it appears, in this DDL and in internal/adapter/mysql — it is otherwise
-- untouched by any repository method today (see ports.go: no
-- MessageRepository exists yet), only written with its column default by
-- CreateGate/ResolveGate's own INSERTs.
CREATE TABLE messages (
    sequence      BIGINT AUTO_INCREMENT PRIMARY KEY, -- BIGSERIAL equivalent, preserves TS's replay-order guarantee
    tenant_id     CHAR(36) NOT NULL,
    from_handle   VARCHAR(255) NOT NULL,
    to_handle     VARCHAR(255) NOT NULL,
    subject       TEXT,
    body          TEXT,
    type          VARCHAR(32) NOT NULL CHECK (type IN
        ('status','dispatch','worker_done','merge_ready','escalation','handoff','decision_gate','heartbeat')),
    thread_id     VARCHAR(255),
    payload       JSON,
    `read`        BOOLEAN NOT NULL DEFAULT FALSE,
    delivered_at  TIMESTAMP(6) NULL,
    created_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
CREATE INDEX idx_messages_to_handle ON messages (tenant_id, to_handle, `read`);
