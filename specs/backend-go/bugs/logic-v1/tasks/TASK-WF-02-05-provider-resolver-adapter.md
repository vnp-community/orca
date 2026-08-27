# TASK-WF-02-05: Implement `ProviderResolver` adapter + wire into `AgentExecutor`

**From Solution:** SOL-WF-02
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/adapter/providerresolver/resolver.go` (new)
**Depends on:** TASK-WF-02-02
**Status:** `[ ]` TODO

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
