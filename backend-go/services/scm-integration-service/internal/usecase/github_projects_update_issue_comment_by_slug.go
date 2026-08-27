package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type UpdateIssueCommentBySlugParams struct {
	TenantID  string
	Provider  domain.ScmProvider
	ItemSlug  string
	CommentID string
	Body      string
}

type UpdateIssueCommentBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewUpdateIssueCommentBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *UpdateIssueCommentBySlug {
	return &UpdateIssueCommentBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *UpdateIssueCommentBySlug) Execute(ctx context.Context, in UpdateIssueCommentBySlugParams) (ProjectComment, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return ProjectComment{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return ProjectComment{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ItemSlug == "" {
		return ProjectComment{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_ITEM_SLUG", "item_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return ProjectComment{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	comment, err := uc.githubProjects.UpdateIssueCommentBySlug(ctx, cred, in.ItemSlug, in.CommentID, in.Body)
	if err != nil {
		return ProjectComment{}, apperrors.New(apperrors.KindInternal, "SCM_UPDATE_ISSUE_COMMENT_BY_SLUG_FAILED", "failed to update issue comment by slug", err)
	}
	return comment, nil
}
