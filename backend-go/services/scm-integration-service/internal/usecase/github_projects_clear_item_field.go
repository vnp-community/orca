package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ClearProjectItemFieldParams struct {
	TenantID    string
	Provider    domain.ScmProvider
	ProjectSlug string
	ItemID      string
	FieldID     string
}

type ClearProjectItemField struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewClearProjectItemField(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ClearProjectItemField {
	return &ClearProjectItemField{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ClearProjectItemField) Execute(ctx context.Context, in ClearProjectItemFieldParams) (ProjectItem, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return ProjectItem{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return ProjectItem{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ProjectSlug == "" {
		return ProjectItem{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_PROJECT_SLUG", "project_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return ProjectItem{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	item, err := uc.githubProjects.ClearProjectItemField(ctx, cred, in.ProjectSlug, in.ItemID, in.FieldID)
	if err != nil {
		return ProjectItem{}, apperrors.New(apperrors.KindInternal, "SCM_CLEAR_PROJECT_ITEM_FIELD_FAILED", "failed to clear project item field", err)
	}
	return item, nil
}
