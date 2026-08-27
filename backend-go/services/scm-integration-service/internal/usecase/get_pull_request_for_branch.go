package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetPullRequestForBranchParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	HeadBranch string
}

type GetPullRequestForBranchResult struct {
	PullRequest domain.PullRequest
	Found       bool
}

type GetPullRequestForBranch struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewGetPullRequestForBranch(credentials CredentialResolver, providers ProviderRegistry) *GetPullRequestForBranch {
	return &GetPullRequestForBranch{credentials: credentials, providers: providers}
}

func (uc *GetPullRequestForBranch) Execute(ctx context.Context, in GetPullRequestForBranchParams) (GetPullRequestForBranchResult, error) {
	if in.TenantID == "" {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.HeadBranch == "" {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_BRANCH", "head_branch is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	pr, found, err := provider.GetPullRequestForBranch(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return GetPullRequestForBranchResult{}, apperrors.New(apperrors.KindInternal, "SCM_GET_PR_FOR_BRANCH_FAILED", "failed to get pull request for branch", err)
	}
	return GetPullRequestForBranchResult{PullRequest: pr, Found: found}, nil
}
