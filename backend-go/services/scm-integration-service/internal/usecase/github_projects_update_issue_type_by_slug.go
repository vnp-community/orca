package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type UpdateIssueTypeBySlugParams struct {
	TenantID  string
	Provider  domain.ScmProvider
	ItemSlug  string
	IssueType string
}

type UpdateIssueTypeBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewUpdateIssueTypeBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *UpdateIssueTypeBySlug {
	return &UpdateIssueTypeBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *UpdateIssueTypeBySlug) Execute(ctx context.Context, in UpdateIssueTypeBySlugParams) (WorkItemDetails, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return WorkItemDetails{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return WorkItemDetails{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ItemSlug == "" {
		return WorkItemDetails{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_ITEM_SLUG", "item_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return WorkItemDetails{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	details, err := uc.githubProjects.UpdateIssueTypeBySlug(ctx, cred, in.ItemSlug, in.IssueType)
	if err != nil {
		return WorkItemDetails{}, apperrors.New(apperrors.KindInternal, "SCM_UPDATE_ISSUE_TYPE_BY_SLUG_FAILED", "failed to update issue type by slug", err)
	}
	return details, nil
}
