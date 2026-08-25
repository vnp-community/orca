package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type ListCreateFieldsInput struct {
	ProjectIDOrKey string
	IssueTypeID    string
	WorkspaceID    string
}

// ListCreateFields is Jira-only — Linear has no dynamic per-issue-type
// create-screen field concept, so this usecase always resolves against
// domain.ProviderJira, never a caller-supplied provider.
type ListCreateFields struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListCreateFields(registry ProviderRegistry, credentials CredentialResolver) *ListCreateFields {
	return &ListCreateFields{registry: registry, credentials: credentials}
}

func (uc *ListCreateFields) Execute(ctx context.Context, in ListCreateFieldsInput) ([]domain.CreateField, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderJira)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for jira", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderJira, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no jira credential available", err)
	}
	fields, err := provider.ListCreateFields(ctx, cred, in.ProjectIDOrKey, in.IssueTypeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_CREATE_FIELDS_FAILED", "failed to list create fields", err)
	}
	return fields, nil
}
