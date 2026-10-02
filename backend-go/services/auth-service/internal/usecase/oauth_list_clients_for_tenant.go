package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// OAuthListClientsForTenant lists the clients that have a status row in the
// tenant (that is, clients that attempted to authorize there or were added by
// an admin).
type OAuthListClientsForTenant struct{ repo OAuthRepository }

func NewOAuthListClientsForTenant(repo OAuthRepository) *OAuthListClientsForTenant {
	return &OAuthListClientsForTenant{repo: repo}
}

func (uc *OAuthListClientsForTenant) Execute(ctx context.Context, tenantID string) ([]domain.OAuthClientView, error) {
	if tenantID == "" {
		return nil, errOAuthInvalidRequest("tenant is required")
	}
	views, err := uc.repo.ListOAuthClientViews(ctx, tenantID)
	if err != nil {
		return nil, errOAuthInternal("failed to list clients", err)
	}
	return views, nil
}
