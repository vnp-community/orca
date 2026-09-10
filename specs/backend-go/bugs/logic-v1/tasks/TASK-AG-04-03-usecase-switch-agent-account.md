# TASK-AG-04-03: `AIProviderResolverClient` adapter + `SwitchAgentAccount` saga usecase

**From Solution:** SOL-AG-04
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpcclient/aiprovider_client.go` (new), `backend-go/services/infra-fleet-service/internal/usecase/switch_agent_account.go` (new), `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-AG-01-02, TASK-AG-02-03, TASK-AG-03-04, TASK-AG-04-02
**Status:** `[x]` DONE — infra-fleet-service's AIProviderResolver grpcclient (+ Dial helper) and SwitchAgentAccount saga usecase implemented exactly as specced; ai-provider-service dial + full main.go wiring (AIProviderServiceAddr config, switchAgentAccountUC passed into grpc.New alongside the StartAgentSession/KillAgentSession/ResumeAgentSession chain). switch_agent_account_test.go covers happy-path-different-account, no-alternate-account, resume-succeeds, kill-fails-aborts-before-resolve, and TASK-AG-04-05's credential-injection-blocker-inherits case — all passing.

---

## Context

`SwitchAgentAccount` is a saga: force-kill the current session (`KillAgentSession`), resolve a replacement account excluding the one just rate-limited (`ai-provider-service.ResolveProvider`), then spawn or resume with the new account. This is `infra-fleet-service`'s first caller of `ai-provider-service`, hence the new outbound gRPC client (deferred from TASK-AG-01-07).

## Changes to make

### `internal/adapter/grpcclient/aiprovider_client.go` (new)

Mirrors `git-gateway-service`'s existing client of the same RPC, extended
with `projectID`/`excludeAccountID`:

```go
// Package grpcclient — infra-fleet-service's outbound client to
// ai-provider-service. NEW dependency edge (infra --> aiprov) on
// 02-microservices-decomposition.md's graph — see TASK-AG-04-03.
package grpcclient

import (
	"context"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// AIProviderResolver implements usecase.AIProviderResolverClient by calling
// ai-provider-service's ResolveProvider RPC directly.
type AIProviderResolver struct {
	client aiproviderv1.AiProviderServiceClient
}

func NewAIProviderResolver(client aiproviderv1.AiProviderServiceClient) *AIProviderResolver {
	return &AIProviderResolver{client: client}
}

func (a *AIProviderResolver) ResolveProvider(ctx context.Context, tenantID, userID, projectID, excludeAccountID string) (providerType, accountID, status string, err error) {
	resp, err := a.client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
		TenantId: tenantID, UserId: userID, ProjectId: projectID, ExcludeAccountId: excludeAccountID,
	})
	if err != nil {
		return "", "", "", err
	}
	account := resp.GetAccount()
	if account == nil {
		return "", "", "", nil
	}
	return account.GetType().String(), account.GetId(), account.GetStatus(), nil
}
```

### `internal/usecase/switch_agent_account.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type SwitchAgentAccountInput struct {
	ConnectionID string
	WorktreeID   string
	UserID       string
	ProjectID    string
	Cwd          string
}

// SwitchAgentAccount — BL-AG-04's saga: force-kill the current session,
// resolve a replacement account (excluding the one just switched away
// from), then resume if BR-AG-09-compatible or start fresh otherwise.
type SwitchAgentAccount struct {
	sessions AgentSessionRepository
	kill     *KillAgentSession
	resolve  AIProviderResolverClient
	start    *StartAgentSession
	resume   *ResumeAgentSession
}

func NewSwitchAgentAccount(sessions AgentSessionRepository, kill *KillAgentSession, resolve AIProviderResolverClient, start *StartAgentSession, resume *ResumeAgentSession) *SwitchAgentAccount {
	return &SwitchAgentAccount{sessions: sessions, kill: kill, resolve: resolve, start: start, resume: resume}
}

func (uc *SwitchAgentAccount) Execute(ctx context.Context, in SwitchAgentAccountInput) (domain.AgentSession, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	found, current, err := uc.sessions.LatestForWorktree(ctx, tenantID, in.WorktreeID)
	if err != nil || !found {
		return domain.AgentSession{}, apperrors.New(apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND", "no running agent session for this worktree", err)
	}

	// Force kill — a switch aborts the current run entirely (BL-AG-04 step 4b).
	if err := uc.kill.Execute(ctx, current.ID, "SIGKILL"); err != nil {
		return domain.AgentSession{}, err // kill failure aborts the switch — do not spawn a second agent on top of a possibly-still-alive one
	}

	// Priority cascade, excluding the account just switched away from —
	// see TASK-AG-04-02.
	_, accountID, _, err := uc.resolve.ResolveProvider(ctx, tenantID, in.UserID, in.ProjectID, current.AccountID)
	if err != nil {
		return domain.AgentSession{}, apperrors.New(apperrors.KindInternal, "INFRA_SWITCH_RESOLVE_FAILED", "failed to resolve a replacement provider account", err)
	}
	if accountID == "" || accountID == current.AccountID {
		return domain.AgentSession{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SWITCH_NO_ALTERNATE_ACCOUNT", "provider resolution returned no usable alternate account", nil)
	}

	// Resume iff BR-AG-09 compatibility holds — delegated entirely to
	// ResumeAgentSession's own checks (expiry, resumable-id-present,
	// version match), not duplicated here. A resume attempt that fails one
	// of those checks falls back to a fresh start rather than propagating
	// the resume error, since a switch's whole point is "get an agent
	// running again," not "resume or nothing."
	if current.ResumeProviderSessionID != "" {
		resumed, err := uc.resume.Execute(ctx, ResumeAgentSessionInput{
			ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID, Cwd: in.Cwd,
		})
		if err == nil {
			return resumed, nil
		}
	}
	return uc.start.Execute(ctx, StartAgentSessionInput{
		ConnectionID: in.ConnectionID, WorktreeID: in.WorktreeID, UserID: in.UserID,
		Cwd: in.Cwd, ModelID: current.ModelID, AccountID: accountID,
	})
}
```

### `cmd/server/main.go`

Dial `ai-provider-service` and construct the resolver + saga:

```go
aiProviderConn, err := grpc.NewClient(cfg.AIProviderServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil {
	logger.Error("failed to dial ai-provider-service", slog.Any("error", err))
	os.Exit(1)
}
defer aiProviderConn.Close()
aiProviderClient := aiproviderv1.NewAiProviderServiceClient(aiProviderConn)
aiProviderResolver := infragrpcclient.NewAIProviderResolver(aiProviderClient)

switchAgentAccountUC := usecase.NewSwitchAgentAccount(agentSessionStore, killAgentSessionUC, aiProviderResolver, startAgentSessionUC, resumeAgentSessionUC)
```

Add `cfg.AIProviderServiceAddr` to `internal/config` following the exact
pattern the service's other outbound service addrs already use (check
`svcconfig`'s existing fields, e.g. however `ORCA_VERSION`/relay-related env
vars are loaded, for the naming convention — likely
`ORCA_AI_PROVIDER_SERVICE_ADDR`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestSwitchAgentAccount -v
```

Add `switch_agent_account_test.go`:
- happy path, no resumable prior session → `kill` then `start.Execute` called with a **different** `accountID` than `current.AccountID`.
- `Resolve` returns the same `accountID` as the one being switched away from (or empty) → `INFRA_SWITCH_NO_ALTERNATE_ACCOUNT`, `start`/`resume` never called.
- resumable prior session, resume succeeds → `resume.Execute` called instead of `start.Execute`.
- resumable prior session, resume fails → falls back to `start.Execute`.
- `kill` fails → the saga aborts before calling `Resolve`/`start` (regression guard against spawning a second agent on top of a possibly-still-alive one).
