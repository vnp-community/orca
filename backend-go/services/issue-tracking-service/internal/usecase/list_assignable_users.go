package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type ListAssignableUsersInput struct {
	Provider       domain.Provider
	ProjectIDOrKey string
	IssueID        string
	WorkspaceID    string
}

type ListAssignableUsers struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListAssignableUsers(registry ProviderRegistry, credentials CredentialResolver) *ListAssignableUsers {
	return &ListAssignableUsers{registry: registry, credentials: credentials}
}

func (uc *ListAssignableUsers) Execute(ctx context.Context, in ListAssignableUsersInput) ([]domain.UserRef, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	users, err := provider.ListAssignableUsers(ctx, cred, in.ProjectIDOrKey, in.IssueID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_ASSIGNABLE_USERS_FAILED", "failed to list assignable users", err)
	}
	return users, nil
}
