package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ListTeamsInput is Linear-only — Execute always resolves
// domain.ProviderLinear.
type ListTeamsInput struct {
	WorkspaceID string
}

type ListTeams struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListTeams(registry ProviderRegistry, credentials CredentialResolver) *ListTeams {
	return &ListTeams{registry: registry, credentials: credentials}
}

func (uc *ListTeams) Execute(ctx context.Context, in ListTeamsInput) ([]domain.Team, error) {
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
	teams, err := provider.ListTeams(ctx, cred, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_TEAMS_FAILED", "failed to list teams", err)
	}
	return teams, nil
}
