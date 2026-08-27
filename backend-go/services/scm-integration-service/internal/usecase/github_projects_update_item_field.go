package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// UpdateProjectItemFieldParams — the gRPC adapter routes this call here
// directly rather than through ProviderRegistry.Resolve, since Projects v2
// has no cross-provider fan-out.
type UpdateProjectItemFieldParams struct {
	TenantID    string
	Provider    domain.ScmProvider
	ProjectSlug string
	ItemID      string
	Field       ProjectFieldValue
}

type UpdateProjectItemField struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewUpdateProjectItemField(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *UpdateProjectItemField {
	return &UpdateProjectItemField{credentials: credentials, githubProjects: githubProjects}
}

func (uc *UpdateProjectItemField) Execute(ctx context.Context, in UpdateProjectItemFieldParams) (ProjectItem, error) {
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
	item, err := uc.githubProjects.UpdateProjectItemField(ctx, cred, in.ProjectSlug, in.ItemID, in.Field)
	if err != nil {
		return ProjectItem{}, apperrors.New(apperrors.KindInternal, "SCM_UPDATE_PROJECT_ITEM_FIELD_FAILED", "failed to update project item field", err)
	}
	return item, nil
}
