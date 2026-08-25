# TASK-184: Implement `ListTerminalSessions`/`GetTerminalAgentStatus`/`InspectTerminalProcess` usecases

**From Solution:** SOL-029 (design part 3a: read/inspect group — backs `terminal.list`/`terminal.agentStatus`/`terminal.isRunningAgent`/`terminal.inspectProcess`)
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `internal/usecase/list_terminal_sessions.go` (new), `internal/usecase/get_terminal_agent_status.go` (new), `internal/usecase/inspect_terminal_process.go` (new)
**Depends on:** TASK-181, TASK-183
**Status:** `[ ]` TODO

---

## Context

These three are the read-only half of the 8 remaining unary lifecycle RPCs
(`SpawnTerminalSession`/`AttachPty` were TASK-182; the mutating half —
resize/kill/stop/wait/focus — is TASK-185). `terminal.isRunningAgent`
deliberately reuses `GetTerminalAgentStatus` rather than getting a second
RPC — BUG-029 lists `terminal.agentStatus` and `terminal.isRunningAgent` as
two frontend methods asking the same underlying question at different
granularity; the `wscompat` layer (TASK-186) is what projects one RPC's
response into two channel shapes, not this usecase.

## Changes to make

### `internal/usecase/list_terminal_sessions.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type ListTerminalSessions struct {
	sessions TerminalSessionRepository
}

func NewListTerminalSessions(sessions TerminalSessionRepository) *ListTerminalSessions {
	return &ListTerminalSessions{sessions: sessions}
}

// Execute lists this tenant's terminal sessions, optionally filtered by
// connectionID (empty = every session for the tenant).
func (uc *ListTerminalSessions) Execute(ctx context.Context, connectionID string) ([]domain.TerminalSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	sessions, err := uc.sessions.ListByConnection(ctx, tenantID, connectionID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TERMINAL_LIST_FAILED", "failed to list terminal sessions", err)
	}
	return sessions, nil
}
```

### `internal/usecase/get_terminal_agent_status.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TerminalAgentStatus struct {
	AgentRunning  bool
	AgentKind     string
	ReadyForInput bool
}

// GetTerminalAgentStatus answers "is an agent CLI alive/ready in this
// pty" — backs BOTH terminal.agentStatus and terminal.isRunningAgent at
// the wscompat layer (TASK-186 projects this one response into two channel
// shapes: isRunningAgent == AgentRunning alone).
type GetTerminalAgentStatus struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
}

func NewGetTerminalAgentStatus(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient) *GetTerminalAgentStatus {
	return &GetTerminalAgentStatus{sessions: sessions, connections: connections, agent: agent}
}

func (uc *GetTerminalAgentStatus) Execute(ctx context.Context, ptyID string) (TerminalAgentStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return TerminalAgentStatus{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return TerminalAgentStatus{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return TerminalAgentStatus{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}
	running, kind, ready, err := uc.agent.AgentStatus(ctx, devServer, ptyID)
	if err != nil {
		return TerminalAgentStatus{}, apperrors.New(apperrors.KindInternal, "TERMINAL_AGENT_STATUS_FAILED", "failed to query agent status", err)
	}
	return TerminalAgentStatus{AgentRunning: running, AgentKind: kind, ReadyForInput: ready}, nil
}
```

### `internal/usecase/inspect_terminal_process.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TerminalProcessInfo struct {
	Known   bool // false = agent couldn't answer — an honest "unknown", not a fabricated zero value
	PID     int32
	Command string
	Cwd     string
}

// InspectTerminalProcess is a best-effort process-introspection read — no
// cited agent file suggests a general process-introspection RPC exists
// (unlike AgentStatus's pty-agent-bridge.ts precedent), so Known:false is
// a first-class case, not an error.
type InspectTerminalProcess struct {
	sessions    TerminalSessionRepository
	connections ConnectionResolver
	agent       DevServerAgentClient
}

func NewInspectTerminalProcess(sessions TerminalSessionRepository, connections ConnectionResolver, agent DevServerAgentClient) *InspectTerminalProcess {
	return &InspectTerminalProcess{sessions: sessions, connections: connections, agent: agent}
}

func (uc *InspectTerminalProcess) Execute(ctx context.Context, ptyID string) (TerminalProcessInfo, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return TerminalProcessInfo{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	session, found, err := uc.sessions.Get(ctx, tenantID, ptyID)
	if err != nil || !found {
		return TerminalProcessInfo{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_SESSION_NOT_FOUND", "terminal session not found", err)
	}
	connected, devServer, _, err := uc.connections.ResolveConnection(ctx, tenantID, session.ConnectionID)
	if err != nil || !connected {
		return TerminalProcessInfo{}, apperrors.New(apperrors.KindNotFound, "TERMINAL_CONNECTION_NOT_FOUND", "connection not found", err)
	}
	known, pid, command, cwd, err := uc.agent.InspectProcess(ctx, devServer, ptyID)
	if err != nil {
		return TerminalProcessInfo{}, apperrors.New(apperrors.KindInternal, "TERMINAL_INSPECT_FAILED", "failed to inspect terminal process", err)
	}
	return TerminalProcessInfo{Known: known, PID: pid, Command: command, Cwd: cwd}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./internal/usecase/...
```
