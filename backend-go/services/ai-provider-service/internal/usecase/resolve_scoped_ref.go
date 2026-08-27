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
	if acc, ok := firstResolvable(accounts, ""); ok {
		return acc, nil
	}
	return domain.ProviderAccount{}, &domain.ErrNoProviderAvailable{Reason: domain.ReasonQuotaOrInactive}
}
