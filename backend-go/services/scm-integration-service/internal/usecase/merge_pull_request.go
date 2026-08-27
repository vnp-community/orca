package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type MergePullRequestParams struct {
	TenantID      string
	Provider      domain.ScmProvider
	Repo          string
	Number        int32
	MergeMethod   string
	CommitTitle   string
	CommitMessage string
}

type MergePullRequestResult struct {
	PullRequest domain.PullRequest
	Merged      bool
	SHA         string
}

type MergePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewMergePullRequest(credentials CredentialResolver, providers ProviderRegistry) *MergePullRequest {
	return &MergePullRequest{credentials: credentials, providers: providers}
}

func (uc *MergePullRequest) Execute(ctx context.Context, in MergePullRequestParams) (MergePullRequestResult, error) {
	if in.TenantID == "" {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Number == 0 {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_NUMBER", "number is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	pr, merged, sha, err := provider.MergePullRequest(ctx, cred, in.Repo, in.Number, MergePullRequestInput{
		MergeMethod:   in.MergeMethod,
		CommitTitle:   in.CommitTitle,
		CommitMessage: in.CommitMessage,
	})
	if err != nil {
		return MergePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_MERGE_PULL_REQUEST_FAILED", "failed to merge pull request", err)
	}
	return MergePullRequestResult{PullRequest: pr, Merged: merged, SHA: sha}, nil
}
