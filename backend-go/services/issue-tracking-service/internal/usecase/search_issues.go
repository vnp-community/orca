package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type SearchIssuesInput struct {
	Provider    domain.Provider
	Query       string
	Limit       int32
	WorkspaceID string
}

type SearchIssues struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewSearchIssues(registry ProviderRegistry, credentials CredentialResolver) *SearchIssues {
	return &SearchIssues{registry: registry, credentials: credentials}
}

func (uc *SearchIssues) Execute(ctx context.Context, in SearchIssuesInput) ([]domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	issues, err := provider.SearchIssues(ctx, cred, in.Query, int(in.Limit))
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_SEARCH_FAILED", "failed to search issues", err)
	}
	return issues, nil
}
