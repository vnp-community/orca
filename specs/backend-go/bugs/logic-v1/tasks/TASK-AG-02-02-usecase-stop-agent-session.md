# TASK-AG-02-02: `StopAgentSession` usecase + `resolveAgentSession` lookup helper

**From Solution:** SOL-AG-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/stop_agent_session.go` (new), `backend-go/services/infra-fleet-service/internal/usecase/agent_session_lookup.go` (new)
**Depends on:** TASK-AG-01-02, TASK-AG-01-05
**Status:** `[ ]` TODO

---

## Context

`StopAgentSession` sends `agent.sendInput('\x03')` — the RPC that actually reaches an agent-spawned PTY, per SOL-AG-02's rationale (a different registry than `pty.sendSignal`/`pty.destroy` operate on). Adds `resolveAgentSession`, the agent-session equivalent of `resolveTerminalSession` (`terminal_session_lookup.go`), resolving through `AgentSession.ConnectionID` the same way.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/agent_session_lookup.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// resolveAgentSession is resolveTerminalSession's (terminal_session_lookup.go)
// agent-session equivalent: fetch the AgentSession row, then resolve its
// stored ConnectionID into a live DevServer. Kept as its own function
// (rather than folded into resolveTerminalSession) because AgentSession and
// TerminalSession are different repository types, not because the
// resolution logic differs.
func resolveAgentSession(ctx context.Context, tenantID, sessionID string, sessions AgentSessionRepository, resolver ConnectionResolver) (domain.AgentSession, domain.DevServer, error) {
	found, session, err := sessions.Get(ctx, tenantID, sessionID)
	if err != nil {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SESSION_LOOKUP_FAILED", "failed to look up agent session", err)
	}
	if !found {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "agent session not found", nil)
	}

	connected, devServer, _, err := resolver.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	if !connected {
		return domain.AgentSession{}, domain.DevServer{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "agent session's connection is no longer bound to a dev server", nil)
	}
	return session, devServer, nil
}
```

Create `backend-go/services/infra-fleet-service/internal/usecase/stop_agent_session.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// StopAgentSession — BR-AG-05: graceful stop is Ctrl+C via agent.sendInput,
// not agent.kill (see KillAgentSession for full teardown). Mirrors
// StopTerminalProcess's shape, swapping SendSignal(SIGINT) for
// SendAgentInput('\x03') because that's the RPC that actually reaches an
// agent-spawned PTY — see agent_methods.go / TASK-AG-01-05's doc comment.
type StopAgentSession struct {
	sessions AgentSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewStopAgentSession(sessions AgentSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient) *StopAgentSession {
	return &StopAgentSession{sessions: sessions, resolver: resolver, agent: agent}
}

func (uc *StopAgentSession) Execute(ctx context.Context, sessionID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, devServer, err := resolveAgentSession(ctx, tenantID, sessionID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}
	if err := uc.agent.SendAgentInput(ctx, devServer, session.PtyID, []byte{0x03}); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STOP_FAILED", "failed to send graceful interrupt to agent pty", err)
	}
	// No status write here — the transition to 'stopped' is driven by the
	// agent's own agent.exited notification, consumed by TASK-AG-05's
	// classifier. Writing 'stopped' here would race a graceful exit that
	// takes a few hundred ms and briefly lie about state.
	return nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestStopAgentSession -v
```

Add `stop_agent_session_test.go` with a fake `DevServerAgentClient`:
asserts `SendAgentInput` is called with exactly `{0x03}` against the
session's `PtyID`, and that `SendSignal`/`KillPty` (the shell-PTY methods)
are **never** called — regression guard against reusing the wrong RPC for
an agent PTY.
