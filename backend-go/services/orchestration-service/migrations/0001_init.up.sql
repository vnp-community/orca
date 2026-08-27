-- orchestration-service owns this database exclusively — no other service
-- reads or writes these tables. See
-- specs/backend-go/architecture/05-data-architecture.md.
CREATE SCHEMA IF NOT EXISTS orchestration;

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_uuid()

CREATE TABLE orchestration.coordinator_runs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    origin_task_id      TEXT NOT NULL, -- logical FK -> task-service.tasks.id (different id space, §2.1) — no SQL FK across databases
    spec                JSONB NOT NULL DEFAULT '{}',
    status              TEXT NOT NULL DEFAULT 'idle' CHECK (status IN ('idle','running','completed','failed')),
    coordinator_handle  TEXT NOT NULL,
    poll_interval_ms    INT NOT NULL DEFAULT 2000,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ
);

ALTER TABLE orchestration.coordinator_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orchestration.coordinator_runs
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- orchestration_tasks is THIS service's own DAG-node id space — the
-- Go/Postgres equivalent of TS's TaskRow, deliberately named
-- orchestration_tasks (not tasks) to avoid colliding with task-service's
-- own tasks table. Do not merge these id spaces (orchestration-service.md §2.1).
CREATE TABLE orchestration.orchestration_tasks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    coordinator_run_id  UUID NOT NULL REFERENCES orchestration.coordinator_runs (id) ON DELETE CASCADE,
    parent_id           UUID REFERENCES orchestration.orchestration_tasks (id),
    origin_task_id      TEXT, -- root row only; logical FK -> task-service.tasks.id
    task_title          TEXT NOT NULL,
    spec                JSONB NOT NULL DEFAULT '{}',
    status              TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','ready','dispatched','completed','failed','blocked')),
    deps                JSONB NOT NULL DEFAULT '[]', -- sibling ids, same coordinator_run_id only — drives promotion
    result              JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ
);
CREATE INDEX idx_otasks_run_status ON orchestration.orchestration_tasks (coordinator_run_id, status);

ALTER TABLE orchestration.orchestration_tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orchestration.orchestration_tasks
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE orchestration.dispatch_contexts (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              UUID NOT NULL,
    -- Nullable: the generated CreateDispatchContextRequest proto message
    -- does not carry an orchestration_task_id — see this service's README
    -- "Known gaps". Kept NOT NULL-able (rather than dropped) so the schema
    -- is ready the moment that proto is extended.
    orchestration_task_id  UUID REFERENCES orchestration.orchestration_tasks (id) ON DELETE CASCADE,
    handle                 TEXT NOT NULL,
    coordinator_run_id     UUID NOT NULL REFERENCES orchestration.coordinator_runs (id) ON DELETE CASCADE,
    status                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','dispatched','completed','failed','circuit_broken')),
    failure_count          INT NOT NULL DEFAULT 0,
    last_failure           TEXT,
    dispatched_at          TIMESTAMPTZ,
    completed_at           TIMESTAMPTZ,
    last_heartbeat_at      TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dispatch_task ON orchestration.dispatch_contexts (orchestration_task_id);
CREATE INDEX idx_dispatch_handle ON orchestration.dispatch_contexts (tenant_id, handle);

ALTER TABLE orchestration.dispatch_contexts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orchestration.dispatch_contexts
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE orchestration.decision_gates (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              UUID NOT NULL,
    orchestration_task_id  UUID NOT NULL REFERENCES orchestration.orchestration_tasks (id) ON DELETE CASCADE,
    -- Additive beyond the design doc's §5 sketch: stores the
    -- dispatch_context_id a gate was created from, purely so
    -- ResolveGateResponse can round-trip it — the generated DecisionGate
    -- proto message carries dispatch_context_id, not orchestration_task_id.
    dispatch_context_id    UUID REFERENCES orchestration.dispatch_contexts (id),
    question               TEXT NOT NULL DEFAULT '',
    options                JSONB NOT NULL DEFAULT '[]',
    status                 TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','resolved','timeout')),
    resolution             TEXT,
    resolved_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_gates_pending ON orchestration.decision_gates (tenant_id, status) WHERE status = 'pending';

ALTER TABLE orchestration.decision_gates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orchestration.decision_gates
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- messages: the coordinator's mailbox. No RPC in the current generated
-- proto (PostMessage/ListMessages/MarkMessageRead) touches this table yet
-- — created per the design doc's data model (§5) for forward-compatibility;
-- see README "Known gaps".
CREATE TABLE orchestration.messages (
    sequence      BIGSERIAL PRIMARY KEY, -- preserves TS's replay-order guarantee
    tenant_id     UUID NOT NULL,
    from_handle   TEXT NOT NULL,
    to_handle     TEXT NOT NULL,
    subject       TEXT,
    body          TEXT,
    type          TEXT NOT NULL CHECK (type IN
        ('status','dispatch','worker_done','merge_ready','escalation','handoff','decision_gate','heartbeat')),
    thread_id     TEXT,
    payload       JSONB,
    read          BOOLEAN NOT NULL DEFAULT false,
    delivered_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_to_handle ON orchestration.messages (tenant_id, to_handle, read);

ALTER TABLE orchestration.messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orchestration.messages
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
