# TASK-AG-01-02: Add `domain.AgentSession` entity and extend `usecase/ports.go` with agent-spawn ports

**From Solution:** SOL-AG-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/agent_session.go` (new), `backend-go/services/infra-fleet-service/internal/usecase/ports.go`
**Depends on:** TASK-AG-01-01
**Status:** `[ ]` TODO

---

## Context

Introduces `AgentSession` as a specialization of `TerminalSession` (references `terminal_sessions.pty_id`, not a replacement) and the ports `StartAgentSession` (TASK-AG-01-08) will depend on: `DevServerAgentClient.SpawnAgent/KillAgent/SendAgentInput`, `AgentSessionRepository`, `AIProviderResolverClient`.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/domain/agent_session.go`:

```go
package domain

import (
	"errors"
	"time"
)

// AgentStatus is an AgentSession's lifecycle state — see
// usecase/agent_output_classifier.go (TASK-AG-05-04) for how transitions
// are detected.
type AgentStatus string

const (
	AgentStatusSpawning AgentStatus = "spawning" // between SpawnAgent accept and first idle signal
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusWaiting   AgentStatus = "waiting"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusError     AgentStatus = "error"
	AgentStatusStopped   AgentStatus = "stopped"
)

// ErrAgentAlreadyRunning is returned by AgentSessionRepository.Create when a
// partial unique constraint (BR-AG-01: one non-terminal agent session per
// worktree+user) rejects the insert.
var ErrAgentAlreadyRunning = errors.New("domain: an agent session is already running for this worktree+user")

// AgentSession is a specialization of TerminalSession — it references
// terminal_sessions.pty_id via PtyID rather than duplicating PTY bookkeeping.
// ConnectionID is the resolution key (mirrors TerminalSession.ConnectionID —
// resolveAgentSession, TASK-AG-02-*, re-resolves through it via
// ConnectionResolver exactly like resolveTerminalSession does); DevServerID
// is a display-only snapshot of the dev server resolved at spawn time, not
// itself used for lookups.
type AgentSession struct {
	ID, PtyID, TenantID, ConnectionID, WorktreeID, DevServerID string
	UserID, ModelID, AccountID                                 string
	ResumeOfSessionID                                          string // "" for a fresh start
	AgentVersion                                                string // dev server's agent_version at spawn time
	Status                                                      AgentStatus
	StartedAt, LastActiveAt                                     time.Time
	StoppedAt                                                   *time.Time
}

// UsesStreamJSON reports whether this session's spawn used
// --output-format stream-json (Claude Code fresh spawns only) — see
// TASK-AG-05-04's two-track classifier for why this matters.
func (s AgentSession) UsesStreamJSON() bool {
	return s.ModelID == "claude" && s.ResumeOfSessionID == ""
}
```

In `usecase/ports.go`, extend the `DevServerAgentClient` interface (after the existing `InspectProcess` method) and add the new port interfaces + input/result types:

```go
	// --- Agent sessions (TASK-AG-01..05) ---
	// SpawnAgent calls agent.spawn. Returns immediately once the agent
	// accepts the request ({ok:true, ptyId}) — output/exit arrive later as
	// agent.output/agent.exited notifications over the same StreamPty
	// subscription used for plain PTYs.
	SpawnAgent(ctx context.Context, devServer domain.DevServer, in SpawnAgentInput) (SpawnAgentResult, error)
	// KillAgent calls agent.kill — signal is "SIGTERM" (graceful) or
	// "SIGKILL" (force).
	KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error
	// SendAgentInput calls agent.sendInput — used for graceful Ctrl+C.
	SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error
}

// SpawnAgentInput mirrors agent.spawn's real param set 1:1
// (agent-spawner.ts's AgentSpawnRequest) — resolvedApiKey intentionally
// absent, see TASK-AG-01-04 (credential injection blocker).
type SpawnAgentInput struct {
	TaskID       string // this service's own session id, minted before calling
	UserID       string
	ModelID      string
	AccountID    string
	Cwd          string
	ResumeID     string // "" for a fresh start; set by ResumeAgentSession
	WorktreePath string
	BranchName   string
	Cols, Rows   int32
	TrustPreset  string
}

type SpawnAgentResult struct {
	PtyID string
}

// AgentSessionRepository persists AgentSession.
type AgentSessionRepository interface {
	// Create enforces BR-AG-01 (one non-terminal agent session per
	// worktree+user) via a partial unique constraint at the DB layer —
	// domain.ErrAgentAlreadyRunning on conflict, not a race-prone
	// check-then-insert.
	Create(ctx context.Context, s domain.AgentSession) (domain.AgentSession, error)
	Get(ctx context.Context, tenantID, sessionID string) (found bool, s domain.AgentSession, err error)
	// LatestForWorktree — SELECT ... ORDER BY started_at DESC LIMIT 1, used
	// by ResumeAgentSession.
	LatestForWorktree(ctx context.Context, tenantID, worktreeID string) (found bool, s domain.AgentSession, err error)
	UpdateStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error
	MarkStopped(ctx context.Context, tenantID, sessionID string, now time.Time) error
}

// AIProviderResolverClient — infra-fleet-service's own client of
// ai-provider-service.ResolveProvider, same RPC
// git-gateway-service/internal/adapter/grpcclient/aiprovider_client.go
// already calls (second caller — a NEW edge on
// 02-microservices-decomposition.md's dependency graph, infra --> aiprov),
// extended with the projectID/excludeAccountID params SwitchAgentAccount
// needs (TASK-AG-04-02's additive ResolveProviderRequest.exclude_account_id
// field). The port is defined here but has no caller until TASK-AG-04-03
// (SwitchAgentAccount) — StartAgentSession itself never calls Resolve, see
// TASK-AG-01-07's context.
type AIProviderResolverClient interface {
	ResolveProvider(ctx context.Context, tenantID, userID, projectID, excludeAccountID string) (providerType, accountID, status string, err error)
}
```

Note the `AIProviderResolverClient` signature matches the real
`ai-provider-service.ResolveProvider` RPC and `grpcclient.AIProviderResolver`
in `git-gateway-service` (`(providerType, accountID, status string, err error)`),
not SOL-AG-01's simplified `(accountID, modelHint, status string, err error)`
sketch — ground truth is the existing client, not the solution doc's gloss.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/internal/domain/... ./services/infra-fleet-service/internal/usecase/...
```

Expected: compiles (interfaces have no implementations yet — `go build` on
the package itself is enough; downstream callers land in later tasks).
