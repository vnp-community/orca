package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListAssignableUsersBySlugParams struct {
	TenantID string
	Provider domain.ScmProvider
	ItemSlug string
}

type ListAssignableUsersBySlug struct {
	credentials    CredentialResolver
	githubProjects GitHubProjectsProvider
}

func NewListAssignableUsersBySlug(credentials CredentialResolver, githubProjects GitHubProjectsProvider) *ListAssignableUsersBySlug {
	return &ListAssignableUsersBySlug{credentials: credentials, githubProjects: githubProjects}
}

func (uc *ListAssignableUsersBySlug) Execute(ctx context.Context, in ListAssignableUsersBySlugParams) ([]AssignableUser, error) {
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
	users, err := uc.githubProjects.ListAssignableUsersBySlug(ctx, cred, in.ItemSlug)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_ASSIGNABLE_USERS_BY_SLUG_FAILED", "failed to list assignable users by slug", err)
	}
	return users, nil
}
