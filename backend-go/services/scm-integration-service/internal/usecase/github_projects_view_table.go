package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ViewProjectTableParams struct {
	TenantID    string
	Provider    domain.ScmProvider
	ProjectSlug string
	ViewID      string
	PageToken   string
	PageSize    int32
}

type ViewProjectTableResult struct {
	Items         []ProjectItem
	NextPageToken string
}

type ViewProjectTable struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewViewProjectTable(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ViewProjectTable {
	return &ViewProjectTable{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ViewProjectTable) Execute(ctx context.Context, in ViewProjectTableParams) (ViewProjectTableResult, error) {
	if in.Provider != domain.ScmProviderGitHub {
		return ViewProjectTableResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "GitHub Projects v2 is not available for this provider", nil)
	}
	if in.TenantID == "" {
		return ViewProjectTableResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.ProjectSlug == "" {
		return ViewProjectTableResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_PROJECT_SLUG", "project_slug is required", nil)
	}
	cred, err := uc.credentials.Resolve(ctx, in.TenantID, domain.ScmProviderGitHub)
	if err != nil {
		return ViewProjectTableResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	items, nextPageToken, err := uc.githubProjects.ViewProjectTable(ctx, cred, in.ProjectSlug, in.ViewID, in.PageToken, in.PageSize)
	if err != nil {
		return ViewProjectTableResult{}, apperrors.New(apperrors.KindInternal, "SCM_VIEW_PROJECT_TABLE_FAILED", "failed to view project table", err)
	}
	return ViewProjectTableResult{Items: items, NextPageToken: nextPageToken}, nil
}
