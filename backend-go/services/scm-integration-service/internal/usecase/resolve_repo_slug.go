package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ResolveRepoSlugParams struct {
	TenantID  string
	Provider  domain.ScmProvider
	Candidate string
}

type ResolveRepoSlugResult struct {
	Owner string
	Name  string
	Slug  string
}

type ResolveRepoSlug struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewResolveRepoSlug(credentials CredentialResolver, providers ProviderRegistry) *ResolveRepoSlug {
	return &ResolveRepoSlug{credentials: credentials, providers: providers}
}

func (uc *ResolveRepoSlug) Execute(ctx context.Context, in ResolveRepoSlugParams) (ResolveRepoSlugResult, error) {
	if in.TenantID == "" {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Candidate == "" {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_CANDIDATE", "candidate is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	owner, name, err := provider.ResolveRepoSlug(ctx, cred, in.Candidate)
	if err != nil {
		return ResolveRepoSlugResult{}, apperrors.New(apperrors.KindInternal, "SCM_RESOLVE_REPO_SLUG_FAILED", "failed to resolve repo slug", err)
	}
	return ResolveRepoSlugResult{Owner: owner, Name: name, Slug: owner + "/" + name}, nil
}
