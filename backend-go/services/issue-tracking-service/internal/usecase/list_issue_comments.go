package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type ListIssueCommentsInput struct {
	Provider    domain.Provider
	IssueID     string
	WorkspaceID string
}

type ListIssueComments struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListIssueComments(registry ProviderRegistry, credentials CredentialResolver) *ListIssueComments {
	return &ListIssueComments{registry: registry, credentials: credentials}
}

func (uc *ListIssueComments) Execute(ctx context.Context, in ListIssueCommentsInput) ([]domain.IssueComment, error) {
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
	comments, err := provider.ListIssueComments(ctx, cred, in.IssueID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_COMMENTS_FAILED", "failed to list comments", err)
	}
	return comments, nil
}
