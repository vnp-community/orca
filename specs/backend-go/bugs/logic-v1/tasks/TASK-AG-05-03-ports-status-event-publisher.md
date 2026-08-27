# TASK-AG-05-03: `AgentStatusEventPublisher` port + `MarkStoppedWithStatus` repository method

**From Solution:** SOL-AG-05
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`, `backend-go/services/infra-fleet-service/internal/adapter/postgres/agent_session_repository.go`
**Depends on:** TASK-AG-01-02, TASK-AG-01-06
**Status:** `[ ]` TODO

---

## Context

`AgentOutputClassifier` (TASK-AG-05-04) needs a publish port for `agent.statusChanged`/`agent:rateLimited`, and a repository method that atomically marks a session stopped with a specific terminal status (`stopped` vs. `error`, decided by exit code) rather than always `stopped` the way `KillAgentSession`'s `MarkStopped` (TASK-AG-01-02) does.

## Changes to make

In `usecase/ports.go`, add:

```go
// AgentStatusEventPublisher publishes agent-session lifecycle events for
// real-time delivery to the renderer (and, per BUG-MB-04, eventually
// mobile) — see TASK-AG-05-05 for the two concrete delivery paths
// (direct in-process push for statusChanged, outbox for rateLimited).
type AgentStatusEventPublisher interface {
	PublishStatusChanged(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus) error
	PublishRateLimited(ctx context.Context, tenantID, sessionID string) error
}
```

Extend `AgentSessionRepository` with:

```go
	// MarkStoppedWithStatus is MarkStopped's exit-driven counterpart — sets
	// a terminal status ('stopped' or 'error', decided by the caller from
	// the pty's exit code) rather than always 'stopped'.
	MarkStoppedWithStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error
```

In `internal/adapter/postgres/agent_session_repository.go` (`AgentSessionStore`, TASK-AG-01-06), add:

```go
func (s *AgentSessionStore) MarkStoppedWithStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.agent_sessions SET status = $1, stopped_at = $2, last_active_at = $2
		WHERE tenant_id = $3 AND id = $4
	`, string(status), now, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("postgres: mark agent session stopped with status: %w", err)
	}
	return nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestAgentSessionStore_MarkStoppedWithStatus -v
```
