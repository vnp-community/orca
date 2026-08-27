package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type GetIssueInput struct {
	Provider    domain.Provider
	IssueID     string
	WorkspaceID string
}

type GetIssue struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewGetIssue(registry ProviderRegistry, credentials CredentialResolver) *GetIssue {
	return &GetIssue{registry: registry, credentials: credentials}
}

func (uc *GetIssue) Execute(ctx context.Context, in GetIssueInput) (domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Issue{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	issue, err := provider.GetIssue(ctx, cred, in.IssueID)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_GET_ISSUE_FAILED", "failed to get issue from provider", err)
	}
	return issue, nil
}
