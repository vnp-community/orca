package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SetPullRequestAutoMergeParams struct {
	TenantID    string
	Provider    domain.ScmProvider
	Repo        string
	Number      int32
	Enabled     bool
	MergeMethod string
}

type SetPullRequestAutoMerge struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewSetPullRequestAutoMerge(credentials CredentialResolver, providers ProviderRegistry) *SetPullRequestAutoMerge {
	return &SetPullRequestAutoMerge{credentials: credentials, providers: providers}
}

func (uc *SetPullRequestAutoMerge) Execute(ctx context.Context, in SetPullRequestAutoMergeParams) (domain.PullRequest, error) {
	if in.TenantID == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	pr, err := provider.SetPullRequestAutoMerge(ctx, cred, in.Repo, in.Number, in.Enabled, in.MergeMethod)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_SET_PR_AUTO_MERGE_FAILED", "failed to set pull request auto-merge", err)
	}
	return pr, nil
}
