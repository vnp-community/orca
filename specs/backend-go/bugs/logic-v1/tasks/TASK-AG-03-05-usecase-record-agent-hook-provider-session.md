# TASK-AG-03-05: `RecordAgentHookProviderSession` — consume `agent.hook`, correlate by worktree (interim fallback)

**From Solution:** SOL-AG-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/record_agent_hook_provider_session.go` (new), `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-AG-03-02, TASK-AG-03-03
**Status:** `[ ]` TODO

---

## Context

`agent.hook` carries no `ptyId`/Orca session id today (TASK-AG-03-07's cross-repo gap), so this usecase implements SOL-AG-03's option 2 interim fallback: correlate a hook event to the most recent non-terminal `AgentSession` for `(tenant_id, worktree_id)`. Correct in the common case (BR-AG-01 already guarantees at most one non-terminal session per worktree+user); wrong only in a narrow race where a hook arrives just after the session left that set.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/record_agent_hook_provider_session.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// RecordAgentHookProviderSession runs once per agent.hook notification
// carrying a providerSession field — subscribed alongside StreamPty's
// existing demux (TASK-AG-03-03), not a new transport. See TASK-AG-03-07
// for why this correlates by worktree rather than a direct session id.
type RecordAgentHookProviderSession struct {
	sessions AgentSessionRepository
}

func NewRecordAgentHookProviderSession(sessions AgentSessionRepository) *RecordAgentHookProviderSession {
	return &RecordAgentHookProviderSession{sessions: sessions}
}

func (uc *RecordAgentHookProviderSession) Handle(ctx context.Context, tenantID string, hook AgentHookEvent) error {
	if hook.ProviderSessionID == "" {
		return nil // most agent.hook events carry no providerSession — not every hook fires it
	}
	found, session, err := uc.sessions.MostRecentActiveForWorktree(ctx, tenantID, hook.WorktreeID)
	if err != nil || !found {
		return err // nothing to attach this to yet — not an error worth failing the notification pump over
	}
	return uc.sessions.UpdateProviderSession(ctx, tenantID, session.ID, hook.ProviderSessionKey, hook.ProviderSessionID)
}

// Run subscribes to devServer's agent.hook stream (DevServerAgentClient.
// StreamAgentHooks, TASK-AG-03-03) and calls Handle for every notification
// until the stream closes or ctx is cancelled. One goroutine per registered
// dev server connection, started lazily by StartAgentSession the first time
// it resolves that dev server — see TASK-AG-03-06 for the idempotent
// per-dev-server-id registry that guards against starting it twice.
func (uc *RecordAgentHookProviderSession) Run(ctx context.Context, tenantID string, devServer domain.DevServer, agent DevServerAgentClient) {
	events, unsubscribe, err := agent.StreamAgentHooks(ctx, devServer)
	if err != nil {
		return
	}
	defer unsubscribe()
	for hook := range events {
		_ = uc.Handle(ctx, tenantID, hook) // best-effort — a correlation miss is not fatal, see Handle's doc comment
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestRecordAgentHookProviderSession -v
```

Add `record_agent_hook_provider_session_test.go`: hook with no
`providerSession` → no-op; hook with one but no active session for the
worktree → no-op, no error; happy path → `UpdateProviderSession` called
with the right key/id.
