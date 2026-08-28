# TASK-AIP-02-05: Thread `DevServerID`/`ModelHint`→`ProviderType` filter through `ResolveProvider`'s cascade

**From Solution:** SOL-AIP-02
**Priority:** P0 — the core correctness fix. This is the change that stops `ResolveProvider` from ever handing back a different provider's account.
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go`
**Depends on:** TASK-AIP-02-02, TASK-AIP-02-03, TASK-AIP-02-04
**Status:** `[x] DONE — DevServerID/ModelHint threaded through all 3 cascade tiers; regression guards (cross-provider at user/project/server tiers with reversed created_at, dev-server scoping) all pass.`

---

## Context

`resolve_provider.go`'s three `ListAccountsFilter{...}` calls
(`resolve_provider.go:52,64,75`) never set `DevServerID` even though the
port and SQL already support it — the concrete cause of the service's own
README-documented gap, "`ResolveProvider`'s server-scope fallback is
tenant-wide rather than per-dev-server." Worse: none of the three tiers
filter by provider type at all, so a tenant with both an Anthropic and an
OpenAI account at the same scope can get either one back depending on
`created_at` ordering — the literal symptom BUG-AIP-02 reports. This task
is the fix: detect the requested provider from `ModelHint` and pass both
`DevServerID` and the detected `ProviderType` into every cascade tier.

## Changes to make

In `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go`,
replace `ResolveProviderInput` and `Resolve`:

```go
type ResolveProviderInput struct {
	UserID      string
	ProjectID   string
	DevServerID string // NEW — threads into every ListAccountsFilter tier
	ModelHint   string // NEW — detected to a ProviderType filter
}

func (uc *ResolveProvider) Resolve(ctx context.Context, in ResolveProviderInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	var providerFilter domain.ProviderType
	if in.ModelHint != "" {
		if p, ok := detectProviderFromModel(in.ModelHint); ok {
			providerFilter = p
		}
	}

	sawAnyCandidate := false

	// Tier 1: user scope — narrowest, wins first.
	if in.UserID != "" {
		accounts, err := uc.repo.List(ctx, ListAccountsFilter{
			TenantID: tenantID, Scope: domain.ScopeUser, ScopeRefID: in.UserID,
			DevServerID: in.DevServerID, ProviderType: providerFilter,
		})
		if err != nil {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_RESOLVE_FAILED", "failed to list user-scope accounts", err)
		}
		if acc, ok := firstResolvable(accounts); ok {
			return acc, nil
		}
		sawAnyCandidate = sawAnyCandidate || len(accounts) > 0
	}

	// Tier 2: project scope.
	if in.ProjectID != "" {
		accounts, err := uc.repo.List(ctx, ListAccountsFilter{
			TenantID: tenantID, Scope: domain.ScopeProject, ScopeRefID: in.ProjectID,
			DevServerID: in.DevServerID, ProviderType: providerFilter,
		})
		if err != nil {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_RESOLVE_FAILED", "failed to list project-scope accounts", err)
		}
		if acc, ok := firstResolvable(accounts); ok {
			return acc, nil
		}
		sawAnyCandidate = sawAnyCandidate || len(accounts) > 0
	}

	// Tier 3: server scope — tenant-wide fallback, now scoped to DevServerID too.
	accounts, err := uc.repo.List(ctx, ListAccountsFilter{
		TenantID: tenantID, Scope: domain.ScopeServer,
		DevServerID: in.DevServerID, ProviderType: providerFilter,
	})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_RESOLVE_FAILED", "failed to list server-scope accounts", err)
	}
	if acc, ok := firstResolvable(accounts); ok {
		return acc, nil
	}
	sawAnyCandidate = sawAnyCandidate || len(accounts) > 0

	reason := domain.ReasonNoScopeMatch
	if sawAnyCandidate {
		reason = domain.ReasonQuotaOrInactive
	}
	return domain.ProviderAccount{}, &domain.ErrNoProviderAvailable{Reason: reason}
}
```

`firstResolvable`/error-reason logic stays untouched — the fix is entirely
in what gets passed to `ListAccountsFilter`, not the cascade's control
flow or ordering.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestResolveProvider
```

Add to `resolve_provider_test.go`:
- **Regression guard for the exact symptom**: seed one Anthropic and one
  OpenAI account both at user scope for the same user, construct the
  OpenAI account with an EARLIER `created_at` than the Anthropic one (to
  prove the fix isn't coincidental); resolve with
  `model_hint="claude-3-5-sonnet"` must return the Anthropic account.
- Same shape at project- and server-scope tiers.
- `TestResolveProvider_DevServerScoping` — two server-scope accounts for
  the same tenant/provider, bound to different `dev_server_id`s; resolving
  with a specific `dev_server_id` must never return the other server's
  account.
- Regression: existing `TestResolveProvider_UserScopeWinsOverProjectScope`
  must still pass unmodified — cascade ordering itself is untouched.
