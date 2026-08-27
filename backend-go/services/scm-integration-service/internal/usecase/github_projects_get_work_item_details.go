package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetWorkItemDetailsBySlugParams struct {
	TenantID string
	Provider domain.ScmProvider
	ItemSlug string
}

type GetWorkItemDetailsBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewGetWorkItemDetailsBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *GetWorkItemDetailsBySlug {
	return &GetWorkItemDetailsBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *GetWorkItemDetailsBySlug) Execute(ctx context.Context, in GetWorkItemDetailsBySlugParams) (WorkItemDetails, error) {
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
	details, err := uc.githubProjects.GetWorkItemDetailsBySlug(ctx, cred, in.ItemSlug)
	if err != nil {
		return WorkItemDetails{}, apperrors.New(apperrors.KindInternal, "SCM_GET_WORK_ITEM_DETAILS_BY_SLUG_FAILED", "failed to get work item details", err)
	}
	return details, nil
}
