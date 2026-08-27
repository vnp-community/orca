package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type ListTeamLabelsInput struct {
	TeamID      string
	WorkspaceID string
}

type ListTeamLabels struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListTeamLabels(registry ProviderRegistry, credentials CredentialResolver) *ListTeamLabels {
	return &ListTeamLabels{registry: registry, credentials: credentials}
}

func (uc *ListTeamLabels) Execute(ctx context.Context, in ListTeamLabelsInput) ([]domain.TeamLabel, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderLinear)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for linear", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderLinear, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no linear credential available", err)
	}
	labels, err := provider.ListTeamLabels(ctx, cred, in.TeamID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_TEAM_LABELS_FAILED", "failed to list team labels", err)
	}
	return labels, nil
}
