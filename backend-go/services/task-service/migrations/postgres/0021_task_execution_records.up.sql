-- Structured outcome of each contract run (CR-REQ-029). execution_link_id has no FK: links are pruned separately.
CREATE TABLE task.task_execution_records (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    task_id           UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    execution_link_id UUID NULL,
    attempt           INT NOT NULL DEFAULT 1,
    spec_digest       CHAR(64) NOT NULL DEFAULT '',
    packet_digest     CHAR(64) NOT NULL DEFAULT '',
    template_version  VARCHAR(16) NOT NULL DEFAULT '',
    parse_status      TEXT NOT NULL CHECK (parse_status IN ('ok','missing','invalid')),
    failure_class     TEXT NULL CHECK (failure_class IN ('retryable','needs_info','spec_defect','env_defect','agent_defect')),
    result            JSONB NULL,
    changes           JSONB NULL,
    stdout_tail       TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_execution_records_task ON task.task_execution_records (tenant_id, task_id, created_at DESC);

ALTER TABLE task.task_execution_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE task.task_execution_records FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_execution_records
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
