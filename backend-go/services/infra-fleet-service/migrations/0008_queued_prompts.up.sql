-- infra.queued_prompts backs SOL-MB-03's DispatchPrompt/GetQueuedPrompt RPCs
-- (proto/orca/infrafleet/v1/infrafleet.proto) — a queued prompt must survive
-- until the agent becomes ready, which (per this service's per-pod
-- connection-ownership caveat, see internal/usecase/attach_pty.go's
-- ptyLiveState doc comment) could outlast the pod that received the
-- original DispatchPrompt call, so this is durable Postgres storage, not the
-- in-memory quiescence registry SOL-MB-02 uses. One row per pty_id enforces
-- BR-MB-12's "overwrite requires confirmation" rule at the usecase layer
-- (this table itself has no uniqueness beyond its primary key to enforce).
CREATE TABLE infra.queued_prompts (
    pty_id                   TEXT PRIMARY KEY REFERENCES infra.terminal_sessions(pty_id) ON DELETE CASCADE,
    tenant_id                UUID NOT NULL,
    prompt                   TEXT NOT NULL,
    dispatched_by_device_id  UUID,
    queued_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE infra.queued_prompts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.queued_prompts
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
