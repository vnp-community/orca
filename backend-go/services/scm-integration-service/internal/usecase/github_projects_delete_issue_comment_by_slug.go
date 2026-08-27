package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type DeleteIssueCommentBySlugParams struct {
	TenantID  string
	Provider  domain.ScmProvider
	ItemSlug  string
	CommentID string
}

type DeleteIssueCommentBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewDeleteIssueCommentBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *DeleteIssueCommentBySlug {
	return &DeleteIssueCommentBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *DeleteIssueCommentBySlug) Execute(ctx context.Context, in DeleteIssueCommentBySlugParams) error {
	if in.Provider != domain.ScmProviderGitHub {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ItemSlug == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_ITEM_SLUG", "item_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	if err := uc.githubProjects.DeleteIssueCommentBySlug(ctx, cred, in.ItemSlug, in.CommentID); err != nil {
		return apperrors.New(apperrors.KindInternal, "SCM_DELETE_ISSUE_COMMENT_BY_SLUG_FAILED", "failed to delete issue comment by slug", err)
	}
	return nil
}
