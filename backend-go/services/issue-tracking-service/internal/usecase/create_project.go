package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type CreateProjectInput struct {
	Provider    domain.Provider
	TeamID      string // Linear
	Name        string
	Description string
	WorkspaceID string
}

type CreateProject struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewCreateProject(registry ProviderRegistry, credentials CredentialResolver) *CreateProject {
	return &CreateProject{registry: registry, credentials: credentials}
}

func (uc *CreateProject) Execute(ctx context.Context, in CreateProjectInput) (domain.ProjectRef, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if in.Name == "" {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_NAME", "name is required", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no credential available for provider", err)
	}
	project, err := provider.CreateProject(ctx, cred, in.WorkspaceID, in.TeamID, in.Name, in.Description)
	if err != nil {
		return domain.ProjectRef{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREATE_PROJECT_FAILED", "failed to create project", err)
	}
	return project, nil
}
