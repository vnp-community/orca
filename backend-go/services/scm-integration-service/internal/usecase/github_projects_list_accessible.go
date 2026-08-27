package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListAccessibleProjectsParams struct {
	TenantID string
	Provider domain.ScmProvider
}

type ListAccessibleProjects struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewListAccessibleProjects(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ListAccessibleProjects {
	return &ListAccessibleProjects{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ListAccessibleProjects) Execute(ctx context.Context, in ListAccessibleProjectsParams) ([]Project, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	projects, err := uc.githubProjects.ListAccessibleProjects(ctx, cred)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_ACCESSIBLE_PROJECTS_FAILED", "failed to list accessible projects", err)
	}
	return projects, nil
}
