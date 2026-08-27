package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// CreatePullRequestParams mirrors CreatePullRequestRequest 1:1 — see
// ListIssuesInput's doc comment for why TenantID is an explicit field here.
// Named *Params (not *Input) to avoid colliding with the port-level
// CreatePullRequestInput in ports.go, which is the narrower shape the
// ScmProvider adapter itself receives.
type CreatePullRequestParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	Title      string
	Body       string
	HeadBranch string
	BaseBranch string
}

// CreatePullRequest resolves this tenant's per-provider credential, resolves
// the concrete provider adapter, and delegates.
type CreatePullRequest struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewCreatePullRequest(credentials CredentialResolver, providers ProviderRegistry) *CreatePullRequest {
	return &CreatePullRequest{credentials: credentials, providers: providers}
}

func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (domain.PullRequest, error) {
	if in.TenantID == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.Title == "" {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_TITLE", "title is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}

	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	pr, err := provider.CreatePullRequest(ctx, cred, in.Repo, CreatePullRequestInput{
		Title:      in.Title,
		Body:       in.Body,
		HeadBranch: in.HeadBranch,
		BaseBranch: in.BaseBranch,
	})
	if err != nil {
		return domain.PullRequest{}, apperrors.New(apperrors.KindInternal, "SCM_CREATE_PULL_REQUEST_FAILED", "failed to create pull request", err)
	}
	return pr, nil
}
