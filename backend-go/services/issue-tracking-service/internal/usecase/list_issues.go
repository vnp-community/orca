package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ListIssuesInput mirrors the ListIssues gRPC request 1:1 by design, minus
// TenantId — tenant comes from context (see this method's Execute), never a
// request body field, per design doc §9's tenant-isolation rule. The proto
// request does carry a tenant_id field (issuetracking.proto predates that
// rule being enforced end-to-end); the gRPC adapter deliberately does not
// forward it here.
type ListIssuesInput struct {
	Provider    domain.Provider
	ProjectKey  string
	FilterJSON  string
	Limit       int32
	WorkspaceID string
}

// ListIssues queries issues from Jira or Linear on the caller's behalf —
// live against the provider, never a cached copy (design doc §2).
type ListIssues struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListIssues(registry ProviderRegistry, credentials CredentialResolver) *ListIssues {
	return &ListIssues{registry: registry, credentials: credentials}
}

func (uc *ListIssues) Execute(ctx context.Context, in ListIssuesInput) ([]domain.Issue, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}

	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}

	issues, err := provider.ListIssues(ctx, cred, in.ProjectKey, in.FilterJSON, int(in.Limit))
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_FAILED", "failed to list issues from provider", err)
	}
	return issues, nil
}
