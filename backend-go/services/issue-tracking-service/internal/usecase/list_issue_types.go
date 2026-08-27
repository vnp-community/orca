package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ListIssueTypesInput is Jira-only — Linear has no issue-type concept — so
// Execute always resolves domain.ProviderJira, never a caller-supplied
// provider field.
type ListIssueTypesInput struct {
	ProjectIDOrKey string
	WorkspaceID    string
}

type ListIssueTypes struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListIssueTypes(registry ProviderRegistry, credentials CredentialResolver) *ListIssueTypes {
	return &ListIssueTypes{registry: registry, credentials: credentials}
}

func (uc *ListIssueTypes) Execute(ctx context.Context, in ListIssueTypesInput) ([]domain.IssueTypeRef, error) {
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
	types, err := provider.ListIssueTypes(ctx, cred, in.ProjectIDOrKey)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_ISSUE_TYPES_FAILED", "failed to list issue types", err)
	}
	return types, nil
}
