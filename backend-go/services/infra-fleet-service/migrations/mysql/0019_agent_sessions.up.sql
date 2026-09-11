-- Extends infra-fleet-service's schema with an AI-agent specialization of
-- terminal_sessions, not a replacement. References
-- terminal_sessions.pty_id (TEXT — agent-assigned, see
-- 0005_terminal_sessions.up.sql's doc comment) so an agent session always
-- has a corresponding PTY-routing row. tenant_id is stored explicitly, same
-- rationale as terminal_sessions: every lookup must join through tenant_id
-- (specs/backend-go/services/infra-fleet-service.md §9).
CREATE TABLE agent_sessions (
    id                    CHAR(36) PRIMARY KEY,
    tenant_id             CHAR(36) NOT NULL,
    -- VARCHAR(255) to match terminal_sessions.pty_id's own MySQL type
    -- (migrations/mysql/0005) — a FK column's type must match what it
    -- references, and InnoDB can't FK into a TEXT/BLOB primary key anyway.
    pty_id                VARCHAR(255) NOT NULL REFERENCES terminal_sessions(pty_id),
    connection_id         CHAR(36) REFERENCES connections(id),  -- resolution key, mirrors terminal_sessions.connection_id
    worktree_id           CHAR(36) NOT NULL,                          -- logical FK -> project-service
    dev_server_id         CHAR(36) NOT NULL REFERENCES dev_servers(id), -- display-only snapshot, not used for lookups
    user_id               CHAR(36) NOT NULL,
    model_id              TEXT NOT NULL,
    account_id            CHAR(36),                                   -- logical FK -> ai_provider.accounts; NULL for localInference
    resume_of_session_id  CHAR(36) REFERENCES agent_sessions(id),
    agent_version         TEXT,                                   -- BR-AG-09, see TASK-AG-03-*
    -- VARCHAR, not TEXT, for this CHECK-constrained enum column — see
    -- migrations/mysql/0013_ephemeral_vm_runtimes.up.sql's comment.
    status                VARCHAR(16) NOT NULL DEFAULT 'spawning' CHECK (status IN
                             ('spawning','idle','running','waiting','completed','error','stopped')),
    started_at            TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_active_at        TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    stopped_at            TIMESTAMP(6)
);

-- BR-AG-01: one non-terminal agent session per worktree+user. MySQL/TiDB
-- has no partial index — same generated-column workaround as
-- migrations/mysql/0002_connections.up.sql's worktree_uniq_key: the key
-- evaluates to NULL (never collides — MySQL, like Postgres, treats multiple
-- NULLs in a UNIQUE index as distinct) once status leaves the active set,
-- and STORED recomputes it on every UPDATE, so this enforces the exact same
-- live invariant as the Postgres partial index, not just at INSERT time.
ALTER TABLE agent_sessions ADD COLUMN active_per_worktree_user_key VARCHAR(600) GENERATED ALWAYS AS (
    CASE WHEN status NOT IN ('stopped', 'completed', 'error')
         THEN CONCAT(tenant_id, '|', worktree_id, '|', user_id) ELSE NULL END
) STORED;
CREATE UNIQUE INDEX idx_infra_agent_sessions_active_per_worktree_user
    ON agent_sessions (active_per_worktree_user_key);

CREATE INDEX idx_infra_agent_sessions_worktree_recent
    ON agent_sessions (tenant_id, worktree_id, started_at DESC); -- resume lookup, TASK-AG-03-*

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.
