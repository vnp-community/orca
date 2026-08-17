-- automation-service owns this database exclusively — no other service
-- reads or writes these tables. See
-- specs/backend-go/architecture/05-data-architecture.md.
CREATE SCHEMA IF NOT EXISTS automation;

-- Automation definitions. Execution is never local — RunNow delegates to
-- workflow-service.ExecuteAdHocStep, see automation-service.md §2. rrule is
-- an RFC 5545 recurrence string, dtstart its anchor time (both required by
-- internal/domain's RecurrenceRule value object).
CREATE TABLE automation.automations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    name                TEXT NOT NULL,
    rrule               TEXT NOT NULL,
    dtstart             TIMESTAMPTZ NOT NULL DEFAULT now(),
    step_config_json    TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_automations_tenant ON automation.automations (tenant_id);

ALTER TABLE automation.automations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation.automations
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Run bookkeeping. status/step_type/output_json/error_message record the
-- REAL outcome workflow-service reported back — unlike TS, where every
-- triggered run resolved skipped_unavailable because no dispatcher was
-- wired (automation-service.md §2/§10).
CREATE TABLE automation.automation_runs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    automation_id       UUID NOT NULL REFERENCES automation.automations (id) ON DELETE CASCADE,
    tenant_id           UUID NOT NULL,
    request_id          TEXT NOT NULL, -- idempotency key, see standards/api-design-guidelines.md
    status              TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    step_type           TEXT NOT NULL DEFAULT '',
    step_config_json    TEXT NOT NULL,
    output_json         TEXT,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,

    -- Idempotency backstop, mirroring usage-service's (tenant_id,
    -- request_id) pattern — a retried or duplicate-ticked RunNow dispatch
    -- for the same occurrence must not create a second row.
    UNIQUE (tenant_id, request_id)
);

CREATE INDEX idx_automation_runs_automation ON automation.automation_runs (automation_id, created_at DESC);

ALTER TABLE automation.automation_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation.automation_runs
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
