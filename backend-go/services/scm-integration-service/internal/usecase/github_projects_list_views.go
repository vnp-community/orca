package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListProjectViewsParams struct {
	TenantID    string
	Provider    domain.ScmProvider
	ProjectSlug string
}

type ListProjectViews struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewListProjectViews(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ListProjectViews {
	return &ListProjectViews{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ListProjectViews) Execute(ctx context.Context, in ListProjectViewsParams) ([]ProjectView, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ProjectSlug == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_PROJECT_SLUG", "project_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	views, err := uc.githubProjects.ListProjectViews(ctx, cred, in.ProjectSlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_PROJECT_VIEWS_FAILED", "failed to list project views", err)
	}
	return views, nil
}
