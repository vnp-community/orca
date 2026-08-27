package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type TestConnectionInput struct {
	Provider    domain.Provider
	WorkspaceID string
}

// TestConnectionResult never errors on an auth failure — false + a message
// IS the answer, per TestConnectionResult's proto shape ({ok, error}).
type TestConnectionResult struct {
	OK    bool
	Error string
}

type TestConnection struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewTestConnection(registry ProviderRegistry, credentials CredentialResolver) *TestConnection {
	return &TestConnection{registry: registry, credentials: credentials}
}

func (uc *TestConnection) Execute(ctx context.Context, in TestConnectionInput) (TestConnectionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return TestConnectionResult{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return TestConnectionResult{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return TestConnectionResult{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return TestConnectionResult{OK: false, Error: "not connected"}, nil
	}
	if _, err := provider.Whoami(ctx, cred); err != nil {
		return TestConnectionResult{OK: false, Error: err.Error()}, nil
	}
	return TestConnectionResult{OK: true}, nil
}
