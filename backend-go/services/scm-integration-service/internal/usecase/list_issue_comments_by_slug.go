package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// ListIssueCommentsBySlugParams mirrors AddIssueCommentBySlugParams' shape
// (Provider is always GitHub today — see that usecase's doc comment: the
// *BySlug RPC group is GitHub Projects v2-only, per GitHubProjectsProvider
// being a separate, narrower port than the common ScmProvider).
type ListIssueCommentsBySlugParams struct {
	TenantID string
	Provider domain.ScmProvider
	ItemSlug string
}

// ListIssueCommentsBySlug completes the *BySlug comment RPC group —
// AddIssueCommentBySlug/UpdateIssueCommentBySlug/DeleteIssueCommentBySlug
// already exist with no way to read the thread back (BUG-PI-01 step 6).
type ListIssueCommentsBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewListIssueCommentsBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ListIssueCommentsBySlug {
	return &ListIssueCommentsBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ListIssueCommentsBySlug) Execute(ctx context.Context, in ListIssueCommentsBySlugParams) ([]ProjectComment, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ItemSlug == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_ITEM_SLUG", "item_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	comments, err := uc.githubProjects.ListIssueCommentsBySlug(ctx, cred, in.ItemSlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_ISSUE_COMMENTS_BY_SLUG_FAILED", "failed to list issue comments", err)
	}
	return comments, nil
}
