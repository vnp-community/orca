# TASK-AG-01-03: Add `agent_sessions` migration

**From Solution:** SOL-AG-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/migrations/0007_agent_sessions.up.sql` (new) + `.down.sql`
**Depends on:** none (can land in parallel with TASK-AG-01-01/02)
**Status:** `[x]` DONE — migrations/0007_agent_sessions.{up,down}.sql added (agent_sessions table, BR-AG-01 partial unique index, worktree-recency index, RLS policy); exercised transitively by agent_session_repository_test.go's `-tags=integration` suite (real Postgres via testcontainers), all passing.

---

## Context

Persists `AgentSession` (TASK-AG-01-02) as an AI-agent specialization of `terminal_sessions`, keyed by `pty_id`. The next available migration number after `0006_browser_profiles` is `0007`.

## Changes to make

Create `backend-go/services/infra-fleet-service/migrations/0007_agent_sessions.up.sql`:

```sql
-- Extends infra-fleet-service's schema with an AI-agent specialization of
-- infra.terminal_sessions, not a replacement. References
-- infra.terminal_sessions.pty_id (TEXT — agent-assigned, see
-- 0005_terminal_sessions.up.sql's doc comment) so an agent session always
-- has a corresponding PTY-routing row. tenant_id is stored explicitly, same
-- rationale as terminal_sessions: every lookup must join through tenant_id
-- (specs/backend-go/services/infra-fleet-service.md §9).
CREATE TABLE infra.agent_sessions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    pty_id                TEXT NOT NULL REFERENCES infra.terminal_sessions(pty_id),
    connection_id         UUID REFERENCES infra.connections(id),  -- resolution key, mirrors terminal_sessions.connection_id
    worktree_id           UUID NOT NULL,                          -- logical FK -> project-service
    dev_server_id         UUID NOT NULL REFERENCES infra.dev_servers(id), -- display-only snapshot, not used for lookups
    user_id               UUID NOT NULL,
    model_id              TEXT NOT NULL,
    account_id            UUID,                                   -- logical FK -> ai_provider.accounts; NULL for localInference
    resume_of_session_id  UUID REFERENCES infra.agent_sessions(id),
    agent_version         TEXT,                                   -- BR-AG-09, see TASK-AG-03-*
    status                TEXT NOT NULL DEFAULT 'spawning' CHECK (status IN
                             ('spawning','idle','running','waiting','completed','error','stopped')),
    started_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    stopped_at            TIMESTAMPTZ
);

-- BR-AG-01: one non-terminal agent session per worktree+user.
CREATE UNIQUE INDEX idx_infra_agent_sessions_active_per_worktree_user
    ON infra.agent_sessions (tenant_id, worktree_id, user_id)
    WHERE status NOT IN ('stopped', 'completed', 'error');

CREATE INDEX idx_infra_agent_sessions_worktree_recent
    ON infra.agent_sessions (tenant_id, worktree_id, started_at DESC); -- resume lookup, TASK-AG-03-*

ALTER TABLE infra.agent_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.agent_sessions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Create `backend-go/services/infra-fleet-service/migrations/0007_agent_sessions.down.sql`:

```sql
DROP TABLE IF EXISTS infra.agent_sessions;
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
# adjust to this repo's actual migration runner if different from golang-migrate:
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" down 1
```

Expected: `up` creates `agent_sessions` and both indexes cleanly against a
database that already has `0001`-`0006` applied (in particular
`terminal_sessions` and `dev_servers` must exist); `down 1` drops it without
error.
