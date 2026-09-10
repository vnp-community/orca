# TASK-AIP-02-06: Add explicit `accountId` (Case 1) and `scoped_ref` (Case 2) resolution

**From Solution:** SOL-AIP-02
**Priority:** P1 — lower-severity gap than the filtering fix (extensions beyond `ai-provider-service.md` §3's literal sketch, flagged in SOL-AIP-02's rationale)
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/resolve_scoped_ref.go` (new)
**Depends on:** TASK-AIP-02-05
**Status:** `[x] DONE — AccountID (Case 1) + ScopedRef (Case 2, resolve_scoped_ref.go) added; TestResolveProvider_ExplicitAccountID + TestResolveScopedRef (table-driven) pass.`

---

## Context

BL-AIP-02 requires two resolution paths with no field in
`ai-provider-service.md` §3's `ResolveRequest` sketch: Case 1, an explicit
`accountId` that bypasses the cascade entirely, and Case 2, a
scope-qualified ref string (`"server:<provider>"` /
`"project:<id>:<provider>"` / `"user:<provider>"`) resolved directly.
`TASK-AIP-02-01` already added `account_id`/`scoped_ref` to the proto;
this task wires the usecase logic.

## Changes to make

In `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go`,
add `AccountID`/`ScopedRef` to `ResolveProviderInput` and short-circuit at
the top of `Resolve`:

```go
type ResolveProviderInput struct {
	UserID      string
	ProjectID   string
	DevServerID string
	ModelHint   string
	AccountID   string // NEW — Case 1, short-circuits the cascade entirely
	ScopedRef   string // NEW — Case 2, parsed then resolved directly
}

func (uc *ResolveProvider) Resolve(ctx context.Context, in ResolveProviderInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	if in.AccountID != "" {
		account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
		if err != nil {
			return domain.ProviderAccount{}, err
		}
		if !account.Resolvable() {
			return domain.ProviderAccount{}, &domain.ErrNoProviderAvailable{Reason: domain.ReasonQuotaOrInactive}
		}
		return account, nil
	}

	if in.ScopedRef != "" {
		return uc.resolveScopedRef(ctx, tenantID, in.ScopedRef)
	}

	// ... existing cascade from TASK-AIP-02-05 unchanged below this point ...
}
```

Create `backend-go/services/ai-provider-service/internal/usecase/resolve_scoped_ref.go`:

```go
package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// resolveScopedRef parses "server:<provider>", "project:<id>:<provider>",
// or "user:<provider>" and resolves directly against that scope — BL-AIP-02
// Case 2, an extension beyond ai-provider-service.md §3's literal
// ResolveRequest sketch (flagged in SOL-AIP-02's rationale).
func (uc *ResolveProvider) resolveScopedRef(ctx context.Context, tenantID, ref string) (domain.ProviderAccount, error) {
	parts := strings.SplitN(ref, ":", 3)
	var scope domain.AccountScope
	var scopeRefID string
	var providerStr string
	switch {
	case len(parts) == 2 && parts[0] == "server":
		scope, providerStr = domain.ScopeServer, parts[1]
	case len(parts) == 2 && parts[0] == "user":
		scope, providerStr = domain.ScopeUser, parts[1]
		// scopeRefID left empty on purpose: "user:<provider>" resolves
		// against the CALLING user, taken from ctx, not embedded in the ref
		// string — mirrors how tenant_id is never trusted from a client body.
	case len(parts) == 3 && parts[0] == "project":
		scope, scopeRefID, providerStr = domain.ScopeProject, parts[1], parts[2]
	default:
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_SCOPED_REF", "unrecognized scoped_ref format: "+ref, nil)
	}
	provider := domain.ProviderType(providerStr)
	if !provider.Valid() {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_INVALID_SCOPED_REF", "unrecognized provider in scoped_ref: "+providerStr, nil)
	}
	if scope == domain.ScopeUser {
		userID, ok := tenant.UserID(ctx)
		if !ok {
			return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_USER", "no user in request context for user:-scoped ref", nil)
		}
		scopeRefID = userID
	}
	accounts, err := uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID, Scope: scope, ScopeRefID: scopeRefID, ProviderType: provider})
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_RESOLVE_FAILED", "failed to list accounts for scoped_ref", err)
	}
	if acc, ok := firstResolvable(accounts); ok {
		return acc, nil
	}
	return domain.ProviderAccount{}, &domain.ErrNoProviderAvailable{Reason: domain.ReasonQuotaOrInactive}
}
```

Confirm `tenant.UserID(ctx)` exists in `common/tenant` (used elsewhere in
this codebase for the same "current user from context" pattern) — if the
helper has a different name, adapt the call to match.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run 'TestResolveProvider_ExplicitAccountID|TestResolveScopedRef'
```

Add:
- `TestResolveProvider_ExplicitAccountID` — bypasses scope entirely;
  inactive/quota-excluded account returns `ErrNoProviderAvailable`, not
  the account.
- `TestResolveScopedRef` — table-driven over `"server:openai"`,
  `"project:<id>:anthropic"`, `"user:google"` (using a
  `tenant.WithUserID`-style test-context helper), and a malformed-string
  rejection case.
