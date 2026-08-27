package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// ListPullRequestsInput mirrors ListPullRequestsRequest 1:1.
type ListPullRequestsInput struct {
	TenantID string
	Provider domain.ScmProvider
	Repo     string
}

// ListPullRequests resolves this tenant's per-provider credential, resolves
// the concrete provider adapter, and delegates.
type ListPullRequests struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewListPullRequests(credentials CredentialResolver, providers ProviderRegistry) *ListPullRequests {
	return &ListPullRequests{credentials: credentials, providers: providers}
}

func (uc *ListPullRequests) Execute(ctx context.Context, in ListPullRequestsInput) ([]domain.PullRequest, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}

	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	prs, err := provider.ListPullRequests(ctx, cred, in.Repo)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_PULL_REQUESTS_FAILED", "failed to list pull requests", err)
	}
	return prs, nil
}
