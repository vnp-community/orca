package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ResolveProjectRefParams struct {
	TenantID string
	Provider domain.ScmProvider
	Owner    string
	Number   int32
}

type ResolveProjectRef struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewResolveProjectRef(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ResolveProjectRef {
	return &ResolveProjectRef{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ResolveProjectRef) Execute(ctx context.Context, in ResolveProjectRefParams) (Project, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return Project{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return Project{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Owner == "" {
		return Project{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_OWNER", "owner is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return Project{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	project, err := uc.githubProjects.ResolveProjectRef(ctx, cred, in.Owner, in.Number)
	if err != nil {
		return Project{}, apperrors.New(apperrors.KindInternal, "SCM_RESOLVE_PROJECT_REF_FAILED", "failed to resolve project ref", err)
	}
	return project, nil
}
