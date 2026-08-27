package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListIssueTypesBySlugParams struct {
	TenantID string
	Provider domain.ScmProvider
	ItemSlug string
}

type ListIssueTypesBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewListIssueTypesBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ListIssueTypesBySlug {
	return &ListIssueTypesBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ListIssueTypesBySlug) Execute(ctx context.Context, in ListIssueTypesBySlugParams) ([]IssueType, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ItemSlug == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_ITEM_SLUG", "item_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	types, err := uc.githubProjects.ListIssueTypesBySlug(ctx, cred, in.ItemSlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_ISSUE_TYPES_BY_SLUG_FAILED", "failed to list issue types by slug", err)
	}
	return types, nil
}
