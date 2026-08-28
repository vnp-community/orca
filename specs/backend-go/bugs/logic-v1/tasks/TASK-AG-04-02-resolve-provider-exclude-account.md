# TASK-AG-04-02: `ResolveProviderInput.ExcludeAccountID` — additive, cascade order unchanged

**From Solution:** SOL-AG-04
**Priority:** P0
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go`, `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`, `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
**Depends on:** none
**Status:** `[x]` DONE — ResolveProviderInput.ExcludeAccountID + firstResolvable's exclude guard implemented; ResolveProviderRequest.exclude_account_id added to aiprovider.proto, threaded through the gRPC handler. resolve_provider_test.go extended with exclude-skips-tier and exclude-all-tiers-no-provider-available cases — all passing, existing cascade-order tests unchanged.

---

## Context

Switching away from a rate-limited account must not immediately re-resolve back to it. `ResolveProviderInput` has no exclusion field today — this adds one, additive only, applied at the `firstResolvable` filtering step so the three-tier cascade **order** (already correct, per BUG-AG-04) is untouched.

## Changes to make

In `resolve_provider.go`, add the field:

```go
type ResolveProviderInput struct {
	UserID           string
	ProjectID        string
	ExcludeAccountID string // "" = no exclusion (existing callers unaffected)
}
```

Change every `firstResolvable(accounts)` call site to
`firstResolvable(accounts, in.ExcludeAccountID)`, and update the helper:

```go
// firstResolvable returns the first account in accounts whose status makes
// it eligible to be handed to a spawn-time caller (domain.ProviderAccount.Resolvable()),
// skipping excludeAccountID if set — see ResolveProviderInput.ExcludeAccountID's
// doc comment. Cascade ORDER (user -> project -> server tier, and within a
// tier, list order) is unchanged by this filter.
func firstResolvable(accounts []domain.ProviderAccount, excludeAccountID string) (domain.ProviderAccount, bool) {
	for _, acc := range accounts {
		if excludeAccountID != "" && acc.ID == excludeAccountID {
			continue
		}
		if acc.Resolvable() {
			return acc, true
		}
	}
	return domain.ProviderAccount{}, false
}
```

Apply the same `excludeAccountID != "" && acc.ID == excludeAccountID`
guard to the `sawAnyCandidate = sawAnyCandidate || len(accounts) > 0`
bookkeeping only if leaving it as-is would misclassify an
all-excluded/all-unresolvable tier as `ReasonQuotaOrInactive` instead of
`ReasonNoScopeMatch` in a way that matters to callers — otherwise leave
`sawAnyCandidate`'s existing `len(accounts) > 0` check unchanged, since an
excluded account still means "a candidate existed in this scope," which is
`ReasonQuotaOrInactive`'s correct meaning (some agent to switch away from
*was* found, it's just not eligible right now).

### Proto + gRPC wiring

`ResolveProviderRequest` (`proto/orca/aiprovider/v1/aiprovider.proto`) needs
a matching field so a caller can actually set the exclusion over the wire:

```protobuf
message ResolveProviderRequest {
  string tenant_id = 1;
  string user_id = 2;
  string project_id = 3;
  string exclude_account_id = 4; // additive — "" = no exclusion, existing callers unaffected
}
```

In `internal/adapter/grpc/server.go`'s `ResolveProvider` handler:

```go
func (s *Server) ResolveProvider(ctx context.Context, req *aiproviderv1.ResolveProviderRequest) (*aiproviderv1.ResolveProviderResponse, error) {
	account, err := s.resolveProvider.Resolve(ctx, usecase.ResolveProviderInput{
		UserID:           req.GetUserId(),
		ProjectID:        req.GetProjectId(),
		ExcludeAccountID: req.GetExcludeAccountId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(mapDomainError(err))
	}
	return &aiproviderv1.ResolveProviderResponse{Account: toProtoAccount(account)}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestResolveProvider -v
```

Extend `resolve_provider_test.go`: `ExcludeAccountID` set to the only
resolvable account in a tier → that tier is skipped and the cascade falls
through to the next tier (or `ErrNoProviderAvailable` if none remain);
existing cascade-order tests unchanged (regression guard the extension
didn't reorder tiers).
