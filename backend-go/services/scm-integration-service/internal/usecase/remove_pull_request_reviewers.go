package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type RemovePullRequestReviewersParams struct {
	TenantID       string
	Provider       domain.ScmProvider
	Repo           string
	Number         int32
	ReviewerLogins []string
}

type RemovePullRequestReviewers struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewRemovePullRequestReviewers(credentials CredentialResolver, providers ProviderRegistry) *RemovePullRequestReviewers {
	return &RemovePullRequestReviewers{credentials: credentials, providers: providers}
}

func (uc *RemovePullRequestReviewers) Execute(ctx context.Context, in RemovePullRequestReviewersParams) (domain.PullRequest, error) {
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
	pr, err := provider.RemovePullRequestReviewers(ctx, cred, in.Repo, in.Number, in.ReviewerLogins)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_REMOVE_PR_REVIEWERS_FAILED", "failed to remove pull request reviewers", err)
	}
	return pr, nil
}
