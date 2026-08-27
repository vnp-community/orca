package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type RequestPullRequestReviewersParams struct {
	TenantID       string
	Provider       domain.ScmProvider
	Repo           string
	Number         int32
	ReviewerLogins []string
	TeamSlugs      []string
}

type RequestPullRequestReviewers struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewRequestPullRequestReviewers(credentials CredentialResolver, providers ProviderRegistry) *RequestPullRequestReviewers {
	return &RequestPullRequestReviewers{credentials: credentials, providers: providers}
}

func (uc *RequestPullRequestReviewers) Execute(ctx context.Context, in RequestPullRequestReviewersParams) (domain.PullRequest, error) {
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
	pr, err := provider.RequestPullRequestReviewers(ctx, cred, in.Repo, in.Number, in.ReviewerLogins, in.TeamSlugs)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_REQUEST_PR_REVIEWERS_FAILED", "failed to request pull request reviewers", err)
	}
	return pr, nil
}
