package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type OAuthSetClientStatusInput struct {
	TenantID    string
	ClientID    string
	Status      domain.OAuthClientStatus // allowed | blocked
	ActorUserID string
}

// OAuthSetClientStatus is the tenant admin's allow/block decision (the admin
// check happens in mcp-service). Blocking also revokes every live token
// family of that client in the tenant, so a block takes effect on tokens
// already issued, not only on future authorizations.
type OAuthSetClientStatus struct {
	repo  OAuthRepository
	audit oauthAuditor
	clock Clock
}

func NewOAuthSetClientStatus(repo OAuthRepository, audit AuditRepository, clock Clock) *OAuthSetClientStatus {
	return &OAuthSetClientStatus{repo: repo, audit: oauthAuditor{audit: audit, clock: clock}, clock: clock}
}

func (uc *OAuthSetClientStatus) Execute(ctx context.Context, in OAuthSetClientStatusInput) (domain.OAuthClientView, error) {
	if in.TenantID == "" {
		return domain.OAuthClientView{}, errOAuthInvalidRequest("tenant is required")
	}
	if in.Status != domain.OAuthClientAllowed && in.Status != domain.OAuthClientBlocked {
		return domain.OAuthClientView{}, errOAuthInvalidRequest("status must be allowed or blocked")
	}
	client, err := lookupOAuthClient(ctx, uc.repo, in.ClientID)
	if err != nil {
		return domain.OAuthClientView{}, err
	}
	prev, err := uc.repo.GetOAuthClientTenantStatus(ctx, in.TenantID, in.ClientID)
	if errors.Is(err, ErrOAuthClientStatusNotFound) {
		return domain.OAuthClientView{}, oauthErr(apperrors.KindNotFound, CodeOAuthClientNotFound, "client is not known in this tenant", nil)
	}
	if err != nil {
		return domain.OAuthClientView{}, errOAuthInternal("failed to read client status", err)
	}
	now := uc.clock.Now()
	next := domain.OAuthClientTenantStatus{TenantID: in.TenantID, ClientID: in.ClientID, Status: in.Status, UpdatedBy: in.ActorUserID, UpdatedAt: now}
	if err := uc.repo.SetOAuthClientTenantStatus(ctx, next); err != nil {
		return domain.OAuthClientView{}, errOAuthInternal("failed to update client status", err)
	}
	revoked := 0
	if in.Status == domain.OAuthClientBlocked {
		if revoked, err = uc.repo.RevokeOAuthFamiliesForClient(ctx, in.TenantID, in.ClientID, domain.OAuthRevokeClientBlocked, now); err != nil {
			return domain.OAuthClientView{}, errOAuthInternal("failed to revoke client tokens", err)
		}
	}
	uc.audit.record(ctx, in.TenantID, in.ActorUserID, "oauth.client_status_changed", "oauth_client", in.ClientID,
		map[string]any{"from": string(prev.Status), "to": string(in.Status), "families_revoked": revoked}, domain.OutcomeAllowed)
	return domain.OAuthClientView{Client: client, Status: next}, nil
}
