# TASK-MB-02-02: Real `ReadyForInput` via output quiescence + `agent_waiting` event + `created_by_user_id` column

**From Solution:** SOL-MB-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_agent_status.go`, `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go`, `backend-go/services/infra-fleet-service/migrations/0007_terminal_session_created_by.up.sql`
**Depends on:** TASK-MB-02-01
**Status:** `[ ]` TODO

---

## Context

`AgentStatus`'s doc comment (`internal/adapter/devserveragent/methods.go:182-196`,
CONFIRMED still current) states `ReadyForInput` is hard-coded equal to
`AgentRunning` because the agent's `pty.listProcesses` surface has no
busy/idle signal. This task improves the signal using ONLY data already in
this process (TASK-MB-02-01's `liveStates` quiescence timestamp) — it does
**not** claim to close the gap fully; a fully accurate signal needs a new
`agent/` RPC (see TASK-MB-02-03).

`TerminalSession` (domain) carries no user identity today — PTY sessions
aren't currently user-scoped beyond tenant — so the lifecycle event payload
(TASK-MB-02-01) has no real recipient to name. This task adds
`created_by_user_id`.

## Changes to make

`backend-go/services/infra-fleet-service/migrations/0007_terminal_session_created_by.up.sql`:

```sql
ALTER TABLE infra.terminal_sessions ADD COLUMN created_by_user_id UUID;
```
(`.down.sql`: `ALTER TABLE infra.terminal_sessions DROP COLUMN created_by_user_id;`)

`backend-go/services/infra-fleet-service/internal/domain/terminal_session.go` — add field:

```go
type TerminalSession struct {
	// ... existing fields unchanged ...
	CreatedByUserID string // threaded from SpawnTerminalSessionInput; empty for pre-migration rows
}
```

Thread `CreatedByUserID` through `SpawnTerminalSessionInput` (populate from
the WS-bridge's already-resolved identity, the same source
`api-gateway.md` §8 already documents for identity propagation), the
`TerminalSessionRepository.Create`/`scanTerminalSession` SQL, and use it in
TASK-MB-02-01's `PublishAgentLifecycle` call site instead of the placeholder
empty slice.

`backend-go/services/infra-fleet-service/internal/usecase/get_terminal_agent_status.go`:

```go
const readyForInputQuiescence = 3 * time.Second // heuristic threshold; tunable, not a business rule

type GetTerminalAgentStatus struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	agent      DevServerAgentClient
	liveStates *sync.Map // shared with AttachPty (TASK-MB-02-01) — same registry instance, injected via cmd/server/main.go
	events     LifecycleEventPublisher
}

func (uc *GetTerminalAgentStatus) Execute(ctx context.Context, ptyID string) (AgentStatusResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return AgentStatusResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return AgentStatusResult{}, err
	}
	result, err := uc.agent.AgentStatus(ctx, devServer, ptyID)
	if err != nil {
		return AgentStatusResult{AgentRunning: false}, nil
	}
	if result.AgentRunning {
		if v, ok := uc.liveStates.Load(ptyID); ok {
			live := v.(*ptyLiveState)
			wasReady := result.ReadyForInput
			result.ReadyForInput = time.Since(live.lastOutputAt) > readyForInputQuiescence
			if result.ReadyForInput && !wasReady {
				_ = uc.events.PublishAgentLifecycle(ctx, tenantID, eventbus.SubjectAgentWaiting, eventbus.AgentLifecyclePayload{
					PtyID: ptyID, ConnectionID: session.ConnectionID, AgentKind: result.AgentKind, UserIDs: []string{session.CreatedByUserID},
				})
			}
		}
		// No live-state entry (cross-pod: the live AttachPty stream for
		// this ptyId runs on a different pod) falls through to AgentStatus's
		// own ReadyForInput == AgentRunning value, unchanged from today — an
		// honest degrade, not a wrong answer.
	}
	return result, nil
}
```

Add `liveStates *sync.Map` and `events LifecycleEventPublisher` to
`NewGetTerminalAgentStatus`'s constructor, and pass the SAME `*sync.Map`
instance TASK-MB-02-01's `AttachPty` writes to (construct it once in
`cmd/server/main.go`, pass by pointer to both usecases).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run GetTerminalAgentStatus
```

Test cases: no live-state entry (cross-pod case) → `ReadyForInput == AgentRunning`,
unchanged (regression guard). A live-state entry with `lastOutputAt` >3s ago
→ `ReadyForInput=true` and `agent_waiting` published exactly once (debounced:
a second poll while still quiescent does NOT republish).
