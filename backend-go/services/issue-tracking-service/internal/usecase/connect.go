package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ConnectInput mirrors ConnectRequest 1:1.
type ConnectInput struct {
	Provider domain.Provider
	SiteURL  string // Jira only
	Email    string // Jira only
	Token    string
}

// Connect verifies cred against the provider BEFORE persisting anything —
// an invalid token must not create a "connected" row a later call then
// fails against (SOL-015's own design note).
type Connect struct {
	registry    ProviderRegistry
	credentials CredentialResolver
	connections ConnectionRepository
}

func NewConnect(registry ProviderRegistry, credentials CredentialResolver, connections ConnectionRepository) *Connect {
	return &Connect{registry: registry, credentials: credentials, connections: connections}
}

func (uc *Connect) Execute(ctx context.Context, in ConnectInput) (domain.ConnectionStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if in.Provider == domain.ProviderJira && in.SiteURL == "" {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_SITE_URL", "site_url is required for jira", nil)
	}
	if in.Token == "" {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_TOKEN", "token is required", nil)
	}

	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}

	cred := Credential{BaseURL: in.SiteURL, Email: in.Email, Token: in.Token}
	viewer, err := provider.Whoami(ctx, cred)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_AUTH_FAILED", "could not authenticate with the provided credential", err)
	}

	credID, err := uc.credentials.Write(ctx, tenantID, userID, in.Provider, cred)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREDENTIAL_WRITE_FAILED", "failed to store credential", err)
	}

	// workspace.ID: Jira sites/Linear workspaces resolve their own real id
	// via a provider-specific "which site/workspace am I" call in a future
	// extension; Connect's minimal contract here only needs an id to
	// upsert against, so it derives one deterministically from the
	// credential when the provider adapter's Whoami doesn't yet resolve a
	// workspace id (Jira's /myself response carries no site id — the site
	// IS the base URL).
	workspace := domain.Workspace{ID: workspaceIDFor(in.Provider, in.SiteURL, viewer.ID), Name: in.SiteURL, URL: in.SiteURL}

	status, err := uc.connections.Upsert(ctx, tenantID, userID, in.Provider, workspace, viewer, credID)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CONNECTION_UPSERT_FAILED", "failed to persist connection", err)
	}
	return status, nil
}

// workspaceIDFor derives a stable workspace id. Jira: the site base URL IS
// the natural unique key (one Atlassian site = one base URL). Linear has a
// single implicit workspace per token in this scaffold — a real
// multi-workspace Linear lookup is a future extension.
func workspaceIDFor(provider domain.Provider, siteURL, viewerID string) string {
	if siteURL != "" {
		return siteURL
	}
	return fmt.Sprintf("%s:%s", provider, viewerID)
}
