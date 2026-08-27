package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetWorkItemDetailsParams struct {
	TenantID string
	Repo     string
	IID      int32
	ItemType string
}

type GetWorkItemDetails struct {
	credentials CredentialResolver
	gitlabMRs   GitLabMergeRequestProvider
}

func NewGetWorkItemDetails(credentials CredentialResolver, gitlabMRs GitLabMergeRequestProvider) *GetWorkItemDetails {
	return &GetWorkItemDetails{credentials: credentials, gitlabMRs: gitlabMRs}
}

func (uc *GetWorkItemDetails) Execute(ctx context.Context, in GetWorkItemDetailsParams) (domain.WorkItemDetailsGitLab, error) {
	if in.TenantID == "" {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitLab)
	if err != nil {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	details, err := uc.gitlabMRs.GetWorkItemDetails(ctx, cred, in.Repo, in.IID, in.ItemType)
	if err != nil {
		return domain.WorkItemDetailsGitLab{}, apperrors.New(apperrors.KindInternal, "SCM_GET_WORK_ITEM_DETAILS_FAILED", "failed to get work item details", err)
	}
	return details, nil
}
