package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ListWorkflowStatesInput backs linear.teamStates — Linear-only, Execute
// always resolves domain.ProviderLinear.
type ListWorkflowStatesInput struct {
	TeamID      string
	WorkspaceID string
}

type ListWorkflowStates struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListWorkflowStates(registry ProviderRegistry, credentials CredentialResolver) *ListWorkflowStates {
	return &ListWorkflowStates{registry: registry, credentials: credentials}
}

func (uc *ListWorkflowStates) Execute(ctx context.Context, in ListWorkflowStatesInput) ([]domain.WorkflowState, error) {
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
	states, err := provider.ListWorkflowStates(ctx, cred, in.TeamID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_WORKFLOW_STATES_FAILED", "failed to list workflow states", err)
	}
	return states, nil
}
