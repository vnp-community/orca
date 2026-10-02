package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type OAuthEnsureClientForTenantInput struct {
	TenantID   string
	ClientID   string
	DCREnabled bool // the tenant's dynamic-registration setting, owned by mcp-service
}

// OAuthEnsureClientForTenant creates the per-tenant status row the first
// time a tenant sees a client: allowed when an admin registered the client
// or the tenant allows DCR, pending otherwise. An existing row is returned
// untouched so a block can never be undone by a new authorize request.
type OAuthEnsureClientForTenant struct {
	repo  OAuthRepository
	clock Clock
}

func NewOAuthEnsureClientForTenant(repo OAuthRepository, clock Clock) *OAuthEnsureClientForTenant {
	return &OAuthEnsureClientForTenant{repo: repo, clock: clock}
}

func (uc *OAuthEnsureClientForTenant) Execute(ctx context.Context, in OAuthEnsureClientForTenantInput) (domain.OAuthClientView, error) {
	if in.TenantID == "" {
		return domain.OAuthClientView{}, errOAuthInvalidRequest("tenant is required")
	}
	client, err := lookupOAuthClient(ctx, uc.repo, in.ClientID)
	if err != nil {
		return domain.OAuthClientView{}, err
	}
	initial := domain.OAuthClientPending
	if client.RegisteredVia == domain.OAuthRegisteredViaAdmin || in.DCREnabled {
		initial = domain.OAuthClientAllowed
	}
	st, err := uc.repo.EnsureOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{
		TenantID: in.TenantID, ClientID: in.ClientID, Status: initial, UpdatedAt: uc.clock.Now(),
	})
	if err != nil {
		return domain.OAuthClientView{}, errOAuthInternal("failed to ensure client status", err)
	}
	return domain.OAuthClientView{Client: client, Status: st}, nil
}
