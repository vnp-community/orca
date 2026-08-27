package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// ResolveProviderInput mirrors the gRPC ResolveProviderRequest. ProjectID
// may be empty (a spawn with no project context still resolves at server
// scope). UserID is required for the user-scope tier — the cascade's
// narrowest tier.
type ResolveProviderInput struct {
	UserID      string
	ProjectID   string
	DevServerID string // threads into every ListAccountsFilter tier
	ModelHint   string // detected to a ProviderType filter
	AccountID   string // Case 1, short-circuits the cascade entirely
	ScopedRef   string // Case 2, parsed then resolved directly
}

// ResolveProvider implements the spawn-time cascade: user-scope wins over
// project-scope wins over server-scope — narrowest scope first. This order
// is fixed by ai-provider-service.md §4, which is explicit that some prior
// TS documentation stated the cascade backwards; the ground truth is
// `ProviderResolver.resolve()` in
// backend/src/main/ai-providers/ProviderResolver.ts, and this ordering
// matches that code. Getting this order right is the entire point of this
// usecase — see resolve_provider_test.go's cascade-order assertions.
//
// Deliberately makes no cross-service call and no credential-broker lookup:
// Resolve reads only this service's own accounts table (§7, §8), which is
// what keeps its p99 < 20ms hot-path latency budget achievable. Returns
// metadata only (id, provider_type, credential_ref) — never a key.
type ResolveProvider struct {
	repo ProviderAccountRepository
}

func NewResolveProvider(repo ProviderAccountRepository) *ResolveProvider {
	return &ResolveProvider{repo: repo}
}

var _ ProviderResolutionPort = (*ResolveProvider)(nil)

func (uc *ResolveProvider) Resolve(ctx context.Context, in ResolveProviderInput) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	// Case 1 — explicit account id bypasses the cascade entirely.
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

	// Case 2 — scope-qualified ref, resolved directly.
	if in.ScopedRef != "" {
		return uc.resolveScopedRef(ctx, tenantID, in.ScopedRef)
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

// firstResolvable returns the first account in accounts whose status makes
// it eligible to be handed to a spawn-time caller (domain.ProviderAccount.Resolvable).
func firstResolvable(accounts []domain.ProviderAccount) (domain.ProviderAccount, bool) {
	for _, acc := range accounts {
		if acc.Resolvable() {
			return acc, true
		}
	}
	return domain.ProviderAccount{}, false
}
