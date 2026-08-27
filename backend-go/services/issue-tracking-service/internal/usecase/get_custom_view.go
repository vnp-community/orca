package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type GetCustomViewInput struct {
	ViewID      string
	Model       string
	WorkspaceID string
}

type GetCustomView struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewGetCustomView(registry ProviderRegistry, credentials CredentialResolver) *GetCustomView {
	return &GetCustomView{registry: registry, credentials: credentials}
}

func (uc *GetCustomView) Execute(ctx context.Context, in GetCustomViewInput) (domain.CustomView, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.CustomView{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.CustomView{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderLinear)
	if err != nil {
		return domain.CustomView{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for linear", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderLinear, in.WorkspaceID)
	if err != nil {
		return domain.CustomView{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no linear credential available", err)
	}
	view, err := provider.GetCustomView(ctx, cred, in.ViewID, in.Model)
	if err != nil {
		return domain.CustomView{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_GET_CUSTOM_VIEW_FAILED", "failed to get custom view", err)
	}
	return view, nil
}
