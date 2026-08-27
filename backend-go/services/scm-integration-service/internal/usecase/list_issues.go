package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// ListIssuesInput mirrors the ListIssuesRequest RPC 1:1 — see
// architecture/03's note that usecase granularity mirrors the RPC surface.
// TenantID arrives as an explicit field (not pulled from grpc context via
// common/tenant, unlike usage-service) because scm-integration-service.md §3
// is explicit that every request already carries a tenant_id resolved
// upstream by api-gateway, as a proto field.
type ListIssuesInput struct {
	TenantID string
	Provider domain.ScmProvider
	Repo     string
}

// ListIssues resolves this tenant's per-provider credential (stubbed, see
// CredentialResolver), resolves the concrete provider adapter from the
// registry, and delegates — the usecase layer's whole job here is correct
// dispatch, not the HTTP call itself.
type ListIssues struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewListIssues(credentials CredentialResolver, providers ProviderRegistry) *ListIssues {
	return &ListIssues{credentials: credentials, providers: providers}
}

func (uc *ListIssues) Execute(ctx context.Context, in ListIssuesInput) ([]domain.Issue, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}

	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	issues, err := provider.ListIssues(ctx, cred, in.Repo, IssueFilter{})
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_ISSUES_FAILED", "failed to list issues", err)
	}
	return issues, nil
}
