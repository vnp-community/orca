package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type CheckHostedReviewEligibilityParams struct {
	TenantID   string
	Provider   domain.ScmProvider
	Repo       string
	HeadBranch string
	BaseBranch string
}

// HostedReviewEligibility mirrors scmintegrationv1.HostedReviewEligibility.
// IneligibleReason is one of "NOT_CONNECTED" | "BRANCH_NOT_FOUND" |
// "REVIEW_ALREADY_EXISTS" (empty when Eligible is true).
type HostedReviewEligibility struct {
	Eligible            bool
	IneligibleReason    string
	ExistingPullRequest domain.PullRequest
}

// CheckHostedReviewEligibility fans out across 3 existing/already-proposed
// capabilities — GetAuthStatus, BranchExists, GetPullRequestForBranch — per
// scm-integration-service.md §6's check_hosted_review_eligibility.go package
// note ("fans out across configured providers"). No ProviderRegistry.For()
// fan-out across multiple providers happens here — "fans out" in the TDD
// refers to fanning out across these 3 checks for the ONE requested
// provider, not iterating every configured provider.
type CheckHostedReviewEligibility struct {
	credentials   CredentialResolver
	providers     ProviderRegistry
	getAuthStatus *GetAuthStatus
}

func NewCheckHostedReviewEligibility(credentials CredentialResolver, providers ProviderRegistry, getAuthStatus *GetAuthStatus) *CheckHostedReviewEligibility {
	return &CheckHostedReviewEligibility{credentials: credentials, providers: providers, getAuthStatus: getAuthStatus}
}

func (uc *CheckHostedReviewEligibility) Execute(ctx context.Context, in CheckHostedReviewEligibilityParams) (HostedReviewEligibility, error) {
	if in.TenantID == "" {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.HeadBranch == "" {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_BRANCH", "head_branch is required", nil)
	}

	// 1. Auth — cheapest check, and every subsequent check is meaningless
	// without it, so fail fast here rather than attempting a branch lookup
	// with no usable credential.
	connected, err := uc.getAuthStatus.Execute(ctx, GetAuthStatusInput{TenantID: in.TenantID, Provider: in.Provider})
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_GET_AUTH_STATUS_FAILED", "failed to check auth status", err)
	}
	if !connected {
		return HostedReviewEligibility{Eligible: false, IneligibleReason: "NOT_CONNECTED"}, nil
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	// 2. Branch existence.
	exists, err := provider.BranchExists(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_BRANCH_EXISTS_CHECK_FAILED", "failed to check branch existence", err)
	}
	if !exists {
		return HostedReviewEligibility{Eligible: false, IneligibleReason: "BRANCH_NOT_FOUND"}, nil
	}

	// 3. Existing open PR/MR for this branch — GetPullRequestForBranch.
	pr, found, err := provider.GetPullRequestForBranch(ctx, cred, in.Repo, in.HeadBranch)
	if err != nil {
		return HostedReviewEligibility{}, apperrors.New(apperrors.KindInternal, "SCM_GET_PR_FOR_BRANCH_FAILED", "failed to check for an existing pull request", err)
	}
	if found {
		return HostedReviewEligibility{Eligible: false, IneligibleReason: "REVIEW_ALREADY_EXISTS", ExistingPullRequest: pr}, nil
	}

	return HostedReviewEligibility{Eligible: true}, nil
}
