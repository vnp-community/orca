package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetLinkedPullRequestsForIssueInput struct {
	TenantID    string
	Provider    domain.ScmProvider
	Repo        string
	IssueNumber int32
}

type GetLinkedPullRequestsForIssueOutput struct {
	PullRequests          []domain.PullRequest
	CapabilityUnsupported bool
}

// GetLinkedPullRequestsForIssue has no *BySlug precedent — provider-generic
// like ListIssues (BUG-PI-01's issue-detail view has no way to see which
// PRs reference an issue). A provider with no cheap "linked PRs" query
// degrades to CapabilityUnsupported=true, never an RPC error.
type GetLinkedPullRequestsForIssue struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewGetLinkedPullRequestsForIssue(credentials CredentialResolver, providers ProviderRegistry) *GetLinkedPullRequestsForIssue {
	return &GetLinkedPullRequestsForIssue{credentials: credentials, providers: providers}
}

func (uc *GetLinkedPullRequestsForIssue) Execute(ctx context.Context, in GetLinkedPullRequestsForIssueInput) (GetLinkedPullRequestsForIssueOutput, error) {
	if in.TenantID == "" {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	prs, supported, err := provider.GetLinkedPullRequestsForIssue(ctx, cred, in.Repo, in.IssueNumber)
	if err != nil {
		return GetLinkedPullRequestsForIssueOutput{}, apperrors.New(apperrors.KindInternal, "SCM_LINKED_PRS_FAILED", "failed to fetch linked pull requests", err)
	}
	if !supported {
		return GetLinkedPullRequestsForIssueOutput{CapabilityUnsupported: true}, nil // degrade, not fail
	}
	return GetLinkedPullRequestsForIssueOutput{PullRequests: prs}, nil
}
