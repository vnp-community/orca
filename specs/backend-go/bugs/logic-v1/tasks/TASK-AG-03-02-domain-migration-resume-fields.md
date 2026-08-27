# TASK-AG-03-02: Extend `agent_sessions` + `domain.AgentSession` with resume-tracking fields

**From Solution:** SOL-AG-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/migrations/0008_agent_sessions_resume.up.sql` (new), `backend-go/services/infra-fleet-service/internal/domain/agent_session.go`, `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
**Depends on:** TASK-AG-01-03
**Status:** `[ ]` TODO

---

## Context

`ResumeAgentSession` needs to know the underlying CLI's *own* conversation/session id (`resume_provider_session_id`) — distinct from `AgentSession.ID` — plus two new domain errors (`ErrAgentSessionExpired`, `ErrAgentVersionMismatch`) and the repository methods `MostRecentActiveForWorktree`/`UpdateProviderSession` the `agent.hook` consumer (TASK-AG-03-05) needs.

## Changes to make

Create `backend-go/services/infra-fleet-service/migrations/0008_agent_sessions_resume.up.sql`:

```sql
ALTER TABLE infra.agent_sessions
  ADD COLUMN resume_provider_session_key TEXT,  -- "session_id" | "conversation_id"
  ADD COLUMN resume_provider_session_id  TEXT;  -- the CLI's OWN id — distinct from agent_sessions.id
```

Create `backend-go/services/infra-fleet-service/migrations/0008_agent_sessions_resume.down.sql`:

```sql
ALTER TABLE infra.agent_sessions
  DROP COLUMN IF EXISTS resume_provider_session_key,
  DROP COLUMN IF EXISTS resume_provider_session_id;
```

In `internal/domain/agent_session.go`, add:

```go
var (
	ErrAgentSessionExpired  = errors.New("domain: agent session has expired (BR-AG-08)")
	ErrAgentVersionMismatch = errors.New("domain: agent version differs from the original session (BR-AG-09)")
)
```

and extend `AgentSession` with the two new fields (append to the struct):

```go
	ResumeProviderSessionKey string // "session_id" | "conversation_id"; "" if never captured
	ResumeProviderSessionID  string // the CLI's OWN session/conversation id — distinct from ID
```

In `usecase/ports.go`, extend `AgentSessionRepository` with:

```go
	// MostRecentActiveForWorktree — the agent.hook correlation fallback
	// (TASK-AG-03-05's "genuine gap" option 2): most recent AgentSession in
	// spawning/running/idle/waiting status for worktreeID. found=false, nil
	// error means none — not an error worth failing the hook-notification
	// pump over.
	MostRecentActiveForWorktree(ctx context.Context, tenantID, worktreeID string) (found bool, s domain.AgentSession, err error)
	// UpdateProviderSession persists the CLI's own resumable session id,
	// captured from an agent.hook notification.
	UpdateProviderSession(ctx context.Context, tenantID, sessionID, providerSessionKey, providerSessionID string) error
```

Also extend `AgentSessionStore` (`internal/adapter/postgres/agent_session_repository.go`,
TASK-AG-01-06) with the matching implementations:

```go
func (s *AgentSessionStore) MostRecentActiveForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	row := s.pool.QueryRow(ctx,
		agentSessionSelect+`WHERE tenant_id = $1 AND worktree_id = $2
		                     AND status IN ('spawning','running','idle','waiting')
		                     ORDER BY started_at DESC LIMIT 1`,
		tenantID, worktreeID)
	session, err := scanAgentSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.AgentSession{}, nil
	}
	if err != nil {
		return false, domain.AgentSession{}, fmt.Errorf("postgres: query most recent active agent session: %w", err)
	}
	return true, session, nil
}

func (s *AgentSessionStore) UpdateProviderSession(ctx context.Context, tenantID, sessionID, providerSessionKey, providerSessionID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.agent_sessions
		SET resume_provider_session_key = $1, resume_provider_session_id = $2
		WHERE tenant_id = $3 AND id = $4
	`, providerSessionKey, providerSessionID, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("postgres: update agent session provider session: %w", err)
	}
	return nil
}
```

Update `agentSessionSelect` and `scanAgentSession` (TASK-AG-01-06) to also
select/scan `COALESCE(resume_provider_session_key, '')` and
`COALESCE(resume_provider_session_id, '')` into the two new
`AgentSession` fields.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" down 1
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestAgentSessionStore -v
```
