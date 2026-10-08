-- One canonical spec per task (CR-REQ-027). Locked rows belong to an approved plan.
CREATE TABLE task.task_specs (
    task_id        UUID PRIMARY KEY REFERENCES task.tasks(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL,
    schema_version INT NOT NULL CHECK (schema_version >= 1),
    spec           JSONB NOT NULL,
    digest         CHAR(64) NOT NULL,
    locked_at      TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    version        BIGINT NOT NULL DEFAULT 1
);
CREATE INDEX idx_task_specs_tenant ON task.task_specs (tenant_id, locked_at);

-- FORCE + NULLIF: a missing app.tenant_id matches nothing instead of erroring or leaking.
ALTER TABLE task.task_specs ENABLE ROW LEVEL SECURITY;
ALTER TABLE task.task_specs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_specs
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
