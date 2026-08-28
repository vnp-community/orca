# TASK-AG-02-03: `KillAgentSession` usecase + nil-safe `WriteActivityChecker` port (BR-AG-06)

**From Solution:** SOL-AG-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/kill_agent_session.go` (new), `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
**Depends on:** TASK-AG-02-02
**Status:** `[x]` DONE — WriteActivityChecker port + KillAgentSession usecase implemented (fail-open, nil-safe); kill_agent_session_test.go covers agent-call-fails-still-marks-stopped, busy-blocks, checker-error-fails-open, and nil-checker-proceeds — all passing.

---

## Context

Full teardown via `agent.kill`, mirroring `KillTerminalSession`'s "mark stopped even if the agent call fails" discipline. BR-AG-06 (don't kill while the agent is mid-file-write) has no existing mechanism anywhere in backend-go — this task wires a `WriteActivityChecker` port that is **nil-safe and fails open** so `KillAgentSession` works today regardless of whether BR-AG-06's tracker (TASK-AG-02-06, a separate open design decision) is ever built.

## Changes to make

In `usecase/ports.go`, add:

```go
// WriteActivityChecker answers "is this worktree mid-file-write right now?"
// — BR-AG-06. No implementation exists in this pass; see TASK-AG-02-06 for
// the open design question on where/whether to build one. A nil
// WriteActivityChecker (or one returning an error) must never block a kill
// — see KillAgentSession.Execute's fail-open handling below.
type WriteActivityChecker interface {
	HasInFlightWrite(ctx context.Context, worktreeID string) (bool, error)
}
```

Create `backend-go/services/infra-fleet-service/internal/usecase/kill_agent_session.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// KillAgentSession — force teardown. Marks the session row 'stopped' even
// if the agent call fails, same discipline as KillTerminalSession.Execute.
type KillAgentSession struct {
	sessions      AgentSessionRepository
	resolver      ConnectionResolver
	agent         DevServerAgentClient
	writeActivity WriteActivityChecker // BR-AG-06 — nil-safe, see Execute
	clock         func() time.Time
}

func NewKillAgentSession(sessions AgentSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient, writeActivity WriteActivityChecker) *KillAgentSession {
	return &KillAgentSession{sessions: sessions, resolver: resolver, agent: agent, writeActivity: writeActivity, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *KillAgentSession) Execute(ctx context.Context, sessionID, signal string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, devServer, err := resolveAgentSession(ctx, tenantID, sessionID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}
	if uc.writeActivity != nil {
		busy, checkErr := uc.writeActivity.HasInFlightWrite(ctx, session.WorktreeID)
		if checkErr == nil && busy {
			// BR-AG-06 — best-effort only. A checker error is NOT treated as
			// "busy" (fail open, not closed — an unreachable checker must
			// never block a user's explicit force-kill request).
			return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS", "cannot kill agent while it is writing a file", nil)
		}
	}
	if signal == "" {
		signal = "SIGKILL"
	}
	agentErr := uc.agent.KillAgent(ctx, devServer, session.PtyID, signal)
	if err := uc.sessions.MarkStopped(ctx, tenantID, sessionID, uc.clock()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_MARK_AGENT_SESSION_STOPPED_FAILED", "failed to mark agent session stopped", err)
	}
	if agentErr != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_KILL_FAILED", "agent session marked stopped, but the dev server agent failed to tear down the pty", agentErr)
	}
	return nil
}
```

Wire `NewKillAgentSession(agentSessionStore, repo, agentClient, nil)` at
`cmd/server/main.go` in TASK-AG-02-04 — pass `nil` for `writeActivity` until
TASK-AG-02-06 (if it's built) supplies a real implementation; the
constructor and `Execute` are both nil-safe by design.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestKillAgentSession -v
```

Add `kill_agent_session_test.go`:
- agent call fails → session still marked stopped (mirrors `kill_terminal_session_test.go`'s "closes even on agent failure" coverage).
- `WriteActivityChecker` reports `busy=true, err=nil` → `KillAgent` never called, `INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS` returned.
- `WriteActivityChecker` returns an error → kill proceeds anyway (fail-open assertion).
- `nil` checker → kill proceeds (BR-AG-06 not wired yet doesn't block basic kill).
