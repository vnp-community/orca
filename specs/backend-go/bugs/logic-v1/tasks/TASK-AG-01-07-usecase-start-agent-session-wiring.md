# TASK-AG-01-07: `StartAgentSession` usecase + gRPC/`main.go` wiring

**From Solution:** SOL-AG-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/start_agent_session.go` (new)
**Depends on:** TASK-AG-01-02, TASK-AG-01-04, TASK-AG-01-05, TASK-AG-01-06
**Status:** `[ ]` TODO

---

## Context

Composes the resolve→spawn→persist shape `SpawnTerminalSession.Execute` already uses, adding BR-AG-01's single-agent-per-worktree-per-user guard and the credential-injection error mapping from TASK-AG-01-04. `account_id` is resolved by the caller *before* this RPC (the renderer already calls the existing `aiProvider.resolve` wscompat channel — see BUG-AG-01's "What backend-go has") — `StartAgentSession` itself makes no cross-service call, so the new `infra --> aiprov` client dial (`AIProviderResolverClient`'s concrete adapter) is deferred to TASK-AG-04-02, the first usecase that actually calls `Resolve`.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/start_agent_session.go`:

```go
package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// StartAgentSessionInput mirrors the gRPC request 1:1.
type StartAgentSessionInput struct {
	ConnectionID string
	WorktreeID   string
	UserID       string
	Cwd          string
	ModelID      string
	AccountID    string
	TrustPreset  string
	ResumeID     string // "" for a fresh start; set by ResumeAgentSession (TASK-AG-03-03)
	Cols, Rows   int32
}

// StartAgentSession spawns an AI-CLI agent via DevServerAgentClient.SpawnAgent
// and persists an AgentSession — follows SpawnTerminalSession.Execute's
// resolve->spawn->persist shape, with BR-AG-01's single-agent guard added.
type StartAgentSession struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
	sessions AgentSessionRepository
	clock    func() time.Time
}

func NewStartAgentSession(resolver ConnectionResolver, agent DevServerAgentClient, sessions AgentSessionRepository) *StartAgentSession {
	return &StartAgentSession{resolver: resolver, agent: agent, sessions: sessions, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *StartAgentSession) Execute(ctx context.Context, in StartAgentSessionInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	connected, devServer, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	if err != nil || !connected {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "no dev server owns this connectionId", err)
	}

	// Minted here, passed as SpawnAgentInput.TaskID — agent.spawn's ptyId
	// embeds this, so the session<->pty linkage is derivable even before
	// the agent's response comes back.
	sessionID := uuid.NewString()

	result, err := uc.agent.SpawnAgent(ctx, devServer, SpawnAgentInput{
		TaskID: sessionID, UserID: in.UserID, ModelID: in.ModelID, AccountID: in.AccountID,
		Cwd: in.Cwd, WorktreePath: in.Cwd, ResumeID: in.ResumeID, Cols: in.Cols, Rows: in.Rows, TrustPreset: in.TrustPreset,
	})
	if err != nil {
		return domain.AgentSession{}, translateAgentSpawnError(err)
	}

	now := uc.clock()
	session, err := uc.sessions.Create(ctx, domain.AgentSession{
		ID: sessionID, PtyID: result.PtyID, TenantID: tenantID, ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID,
		DevServerID: devServer.ID, UserID: in.UserID, ModelID: in.ModelID, AccountID: in.AccountID,
		Status: domain.AgentStatusSpawning, StartedAt: now, LastActiveAt: now,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAgentAlreadyRunning) {
			// The agent process is now orphaned on the dev server — kill it
			// rather than leave an untracked PTY running.
			_ = uc.agent.KillAgent(ctx, devServer, result.PtyID, "SIGKILL")
			return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_ALREADY_RUNNING", "an agent is already running for this worktree and user", err)
		}
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_AGENT_SESSION_FAILED", "failed to persist agent session", err)
	}
	return session, nil
}

// translateAgentSpawnError — see TASK-AG-01-04 for why this mapping exists.
func translateAgentSpawnError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no plaintext resolvedApiKey was provided") ||
		strings.Contains(msg, "no credential found for accountId=") {
		return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE",
			"credential injection for this provider account is not available yet (TASK-AG-01-04)", err)
	}
	return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_SPAWN_AGENT_FAILED", "failed to spawn agent on dev server agent", err)
}
```

### gRPC wiring (`internal/adapter/grpc/server.go`)

Add a `startAgentSession *usecase.StartAgentSession` field to `Server`, thread
it through the constructor (same list as `spawnTerminalSession`), and add:

```go
// --- Agent sessions (TASK-AG-01) ---

func (s *Server) StartAgentSession(ctx context.Context, req *infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
	session, err := s.startAgentSession.Execute(ctx, usecase.StartAgentSessionInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		UserID:       req.GetUserId(),
		Cwd:          req.GetCwd(),
		ModelID:      req.GetModelId(),
		AccountID:    req.GetAccountId(),
		TrustPreset:  req.GetTrustPreset(),
		Cols:         req.GetCols(),
		Rows:         req.GetRows(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentSession(session), nil
}

func toProtoAgentSession(s domain.AgentSession) *infrafleetv1.AgentSession {
	return &infrafleetv1.AgentSession{
		Id: s.ID, PtyId: s.PtyID, ConnectionId: s.ConnectionID, WorktreeId: s.WorktreeID, DevServerId: s.DevServerID,
		UserId: s.UserID, ModelId: s.ModelID, AccountId: s.AccountID, Status: string(s.Status),
		StartedAtUnixMs: s.StartedAt.UnixMilli(), LastActiveAtUnixMs: s.LastActiveAt.UnixMilli(),
	}
}
```

### `cmd/server/main.go` wiring

After the existing `attachPtyUC := usecase.NewAttachPty(...)` line, add:

```go
agentSessionStore := postgres.NewAgentSessionStore(pool)
startAgentSessionUC := usecase.NewStartAgentSession(repo, agentClient, agentSessionStore)
```

Pass `startAgentSessionUC` into `grpc.NewServer(...)`'s existing constructor
call alongside `spawnTerminalSessionUC` et al.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestStartAgentSession -v
```

Add `start_agent_session_test.go` with fake `ConnectionResolver`/
`DevServerAgentClient`/`AgentSessionRepository`:
- resolved connection → `SpawnAgent` called with the right params → session persisted with `spawning` status.
- `domain.ErrAgentAlreadyRunning` from the repository → asserts `KillAgent` is called (cleanup) and the returned error is `INFRA_AGENT_ALREADY_RUNNING`.
- `SpawnAgent` returns the agent's "no plaintext resolvedApiKey" error string → asserts the mapped error is `INFRA_AGENT_CREDENTIAL_INJECTION_UNAVAILABLE` (see TASK-AG-01-04).
- unresolved connection → `INFRA_CONNECTION_NOT_FOUND`, `SpawnAgent` never called.
