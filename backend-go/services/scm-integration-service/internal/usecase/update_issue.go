package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type UpdateIssueParams struct {
	TenantID string
	Provider domain.ScmProvider
	Repo     string
	Number   int32
	Patch    IssuePatch
}

type UpdateIssue struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewUpdateIssue(credentials CredentialResolver, providers ProviderRegistry) *UpdateIssue {
	return &UpdateIssue{credentials: credentials, providers: providers}
}

func (uc *UpdateIssue) Execute(ctx context.Context, in UpdateIssueParams) (domain.Issue, error) {
	if in.TenantID == "" {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}
	issue, err := provider.UpdateIssue(ctx, cred, in.Repo, in.Number, in.Patch)
	if err != nil {
		return domain.Issue{}, apperrors.New(apperrors.KindInternal, "SCM_UPDATE_ISSUE_FAILED", "failed to update issue", err)
	}
	return issue, nil
}
