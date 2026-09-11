-- queued_prompts backs SOL-MB-03's DispatchPrompt/GetQueuedPrompt RPCs
-- (proto/orca/infrafleet/v1/infrafleet.proto) — a queued prompt must survive
-- until the agent becomes ready, which (per this service's per-pod
-- connection-ownership caveat, see internal/usecase/attach_pty.go's
-- ptyLiveState doc comment) could outlast the pod that received the
-- original DispatchPrompt call, so this is durable Postgres storage, not the
-- in-memory quiescence registry SOL-MB-02 uses. One row per pty_id enforces
-- BR-MB-12's "overwrite requires confirmation" rule at the usecase layer
-- (this table itself has no uniqueness beyond its primary key to enforce).
-- pty_id is VARCHAR(255) to match terminal_sessions.pty_id's MySQL type
-- (migrations/mysql/0005) — see that file's comment.
CREATE TABLE queued_prompts (
    pty_id                   VARCHAR(255) PRIMARY KEY REFERENCES terminal_sessions(pty_id) ON DELETE CASCADE,
    tenant_id                CHAR(36) NOT NULL,
    prompt                   TEXT NOT NULL,
    dispatched_by_device_id  CHAR(36),
    queued_at                TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.
