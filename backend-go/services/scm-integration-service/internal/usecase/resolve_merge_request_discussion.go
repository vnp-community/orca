package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ResolveMergeRequestDiscussionParams struct {
	TenantID        string
	Repo            string
	MergeRequestIID int32
	DiscussionID    string
	Resolved        bool
}

type ResolveMergeRequestDiscussion struct {
	credentials CredentialResolver
	gitlabMRs   GitLabMergeRequestProvider
}

func NewResolveMergeRequestDiscussion(credentials CredentialResolver, gitlabMRs GitLabMergeRequestProvider) *ResolveMergeRequestDiscussion {
	return &ResolveMergeRequestDiscussion{credentials: credentials, gitlabMRs: gitlabMRs}
}

func (uc *ResolveMergeRequestDiscussion) Execute(ctx context.Context, in ResolveMergeRequestDiscussionParams) (domain.MergeRequestDiscussion, error) {
	if in.TenantID == "" {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	if in.DiscussionID == "" {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_DISCUSSION_ID", "discussion_id is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitLab)
	if err != nil {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	disc, err := uc.gitlabMRs.ResolveDiscussion(ctx, cred, in.Repo, in.MergeRequestIID, in.DiscussionID, in.Resolved)
	if err != nil {
		return domain.MergeRequestDiscussion{}, apperrors.New(apperrors.KindInternal, "SCM_RESOLVE_MR_DISCUSSION_FAILED", "failed to resolve merge request discussion", err)
	}
	return disc, nil
}
