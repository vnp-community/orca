package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// GetProjectStatusOrderInput is Jira-only — Kanban column grouping has no
// Linear analog — Execute always resolves domain.ProviderJira.
type GetProjectStatusOrderInput struct {
	ProjectIDOrKey string
	WorkspaceID    string
}

type GetProjectStatusOrder struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewGetProjectStatusOrder(registry ProviderRegistry, credentials CredentialResolver) *GetProjectStatusOrder {
	return &GetProjectStatusOrder{registry: registry, credentials: credentials}
}

func (uc *GetProjectStatusOrder) Execute(ctx context.Context, in GetProjectStatusOrderInput) (domain.ProjectStatusOrder, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectStatusOrder{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ProjectStatusOrder{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderJira)
	if err != nil {
		return domain.ProjectStatusOrder{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for jira", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderJira, in.WorkspaceID)
	if err != nil {
		return domain.ProjectStatusOrder{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no jira credential available", err)
	}
	order, err := provider.GetProjectStatusOrder(ctx, cred, in.ProjectIDOrKey)
	if err != nil {
		return domain.ProjectStatusOrder{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_GET_STATUS_ORDER_FAILED", "failed to get project status order", err)
	}
	return order, nil
}
