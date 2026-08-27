package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListMergeRequestsParams struct {
	TenantID     string
	Repo         string
	State        string
	SourceBranch string
}

type ListMergeRequests struct {
	credentials CredentialResolver
	gitlabMRs   GitLabMergeRequestProvider
}

func NewListMergeRequests(credentials CredentialResolver, gitlabMRs GitLabMergeRequestProvider) *ListMergeRequests {
	return &ListMergeRequests{credentials: credentials, gitlabMRs: gitlabMRs}
}

func (uc *ListMergeRequests) Execute(ctx context.Context, in ListMergeRequestsParams) ([]domain.MergeRequest, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitLab)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	mrs, err := uc.gitlabMRs.ListMergeRequests(ctx, cred, in.Repo, MRFilter{State: in.State, SourceBranch: in.SourceBranch})
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_MERGE_REQUESTS_FAILED", "failed to list merge requests", err)
	}
	return mrs, nil
}
