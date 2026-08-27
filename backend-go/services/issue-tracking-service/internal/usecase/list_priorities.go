package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ListPrioritiesInput is Jira-only — global priorities have no Linear
// analog in this proto — Execute always resolves domain.ProviderJira.
type ListPrioritiesInput struct {
	WorkspaceID string
}

type ListPriorities struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListPriorities(registry ProviderRegistry, credentials CredentialResolver) *ListPriorities {
	return &ListPriorities{registry: registry, credentials: credentials}
}

func (uc *ListPriorities) Execute(ctx context.Context, in ListPrioritiesInput) ([]domain.PriorityRef, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderJira)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for jira", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderJira, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no jira credential available", err)
	}
	priorities, err := provider.ListPriorities(ctx, cred)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_PRIORITIES_FAILED", "failed to list priorities", err)
	}
	return priorities, nil
}
