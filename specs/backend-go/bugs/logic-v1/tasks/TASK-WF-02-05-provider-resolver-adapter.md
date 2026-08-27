# TASK-WF-02-05: Implement `ProviderResolver` adapter + wire into `AgentExecutor`

**From Solution:** SOL-WF-02
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/adapter/providerresolver/resolver.go` (new)
**Depends on:** TASK-WF-02-02
**Status:** `[x]` DONE — new `internal/adapter/providerresolver` package: pinned-account path validates active status via `ListAccounts` (errors, never falls back to cascade, on inactive/unknown/list-failure); unpinned path delegates to `ResolveProvider`, erroring if it resolves no account (rather than silently returning ""). Wired into `AgentExecutor` (`agentExecParams` gained `AccountID`/`Model`); `cmd/server/main.go` dials `ai-provider-service` (new `AI_PROVIDER_SERVICE_ADDR` config) and wires `providerresolver.New`. Since `TASK-WF-02-06`'s `ExecutionContext` (this task's stated source for userID/projectID) doesn't exist yet in dependency order, added `tenant.WithProjectID`/`tenant.ProjectID` to `common/tenant` (mirroring existing `WithUserID`/`UserID`) and read both via `ctx` in `AgentExecutor.Execute` — best-effort empty today, will be populated once TASK-WF-02-06's waveDispatcher enriches the dispatch ctx (documented in code). New `resolver_test.go`: pin-wins-over-cascade, pinned-inactive/unknown-errors-without-fallback, list-failure-propagates, no-pin-delegates-with-right-scope, cascade-no-account-errors, empty-pin-treated-as-no-pin (7/7 pass). `go build/vet/test` green for workflow-service, api-gateway, git-gateway-service, project-service (the last two build against `common/tenant`, confirming the additive change is safe).

---

## Context

BUG-WF-02 finds AI provider selection never called at all —
`ai-provider-service`'s `ResolveProvider` cascade and its
pin-beats-cascade rule (`workflow-service.md` §7) are unimplemented.

## Changes to make

Create `backend-go/services/workflow-service/internal/adapter/providerresolver/resolver.go`:

```go
package providerresolver

type resolver struct {
    aiprovider aiproviderv1.AIProviderServiceClient
}

func New(aiprovider aiproviderv1.AIProviderServiceClient) usecase.ProviderResolver {
    return &resolver{aiprovider: aiprovider}
}

func (r *resolver) Resolve(ctx context.Context, tenantID, userID, projectID string, pin *domain.ProviderPin) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    if pin != nil && pin.AccountID != "" {
        // Explicit pin — validate active per workflow-service.md §7's
        // "(validated active)" parenthetical.
        accounts, err := r.aiprovider.ListAccounts(ctx, &aiproviderv1.ListAccountsRequest{TenantId: tenantID})
        if err != nil {
            return "", err
        }
        for _, a := range accounts.GetAccounts() {
            if a.GetId() == pin.AccountID {
                if a.GetStatus() != "active" {
                    return "", fmt.Errorf("providerresolver: pinned account %s is not active (status=%s)", pin.AccountID, a.GetStatus())
                }
                return a.GetId(), nil
            }
        }
        return "", fmt.Errorf("providerresolver: pinned account %s not found", pin.AccountID)
    }

    // No pin — delegate to ai-provider-service's own priority cascade.
    resp, err := r.aiprovider.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
        TenantId: tenantID, UserId: userID, ProjectId: projectID,
    })
    if err != nil {
        return "", err
    }
    return resp.GetAccount().GetId(), nil
}
```

Wire into `AgentExecutor.Execute` (`agent_step_executor.go`): call this
resolver before the relay call, threading the resolved `accountId` (plus
`cfg.Model`, passed through untouched) into `agentExecParams`, extending
that struct with `AccountID`/`Model` fields. `userID`/`projectID` come
from the `ExecutionContext` added in TASK-WF-02-06.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/providerresolver/...
```

Expected: pinned active account wins over cascade; pinned
inactive/unknown account errors without silently falling back to the
cascade; no pin delegates to `ResolveProvider` with the right
tenant/user/project.
