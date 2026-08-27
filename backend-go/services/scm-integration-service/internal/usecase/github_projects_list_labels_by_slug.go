package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListLabelsBySlugParams struct {
	TenantID string
	Provider domain.ScmProvider
	ItemSlug string
}

type ListLabelsBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewListLabelsBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ListLabelsBySlug {
	return &ListLabelsBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ListLabelsBySlug) Execute(ctx context.Context, in ListLabelsBySlugParams) ([]Label, error) {
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
	labels, err := uc.githubProjects.ListLabelsBySlug(ctx, cred, in.ItemSlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_LABELS_BY_SLUG_FAILED", "failed to list labels by slug", err)
	}
	return labels, nil
}
