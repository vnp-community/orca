package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type OAuthRevokeGrantInput struct {
	TenantID    string
	GrantID     string
	Reason      string // user_revoked (default) | admin_revoked
	ActorUserID string
}

// OAuthRevokeGrant cuts every token family of a grant. Idempotent, so
// mcp-service can retry it from its reconcile job. Only mcp-service calls it.
type OAuthRevokeGrant struct {
	repo  OAuthRepository
	audit oauthAuditor
	clock Clock
}

func NewOAuthRevokeGrant(repo OAuthRepository, audit AuditRepository, clock Clock) *OAuthRevokeGrant {
	return &OAuthRevokeGrant{repo: repo, audit: oauthAuditor{audit: audit, clock: clock}, clock: clock}
}

func (uc *OAuthRevokeGrant) Execute(ctx context.Context, in OAuthRevokeGrantInput) error {
	if in.TenantID == "" {
		return errOAuthInvalidRequest("tenant is required")
	}
	if _, err := uuid.Parse(in.GrantID); err != nil {
		return errOAuthInvalidRequest("grant_id must be a UUID")
	}
	reason := in.Reason
	switch reason {
	case "":
		reason = domain.OAuthRevokeUserRevoked
	case domain.OAuthRevokeUserRevoked, domain.OAuthRevokeAdminRevoked:
	default:
		return errOAuthInvalidRequest("unsupported revoke reason")
	}
	n, err := uc.repo.RevokeOAuthGrant(ctx, in.TenantID, in.GrantID, reason, uc.clock.Now())
	if err != nil {
		return errOAuthInternal("failed to revoke grant", err)
	}
	uc.audit.record(ctx, in.TenantID, in.ActorUserID, "oauth.grant_revoked", "oauth_grant", in.GrantID,
		map[string]any{"reason": reason, "families_revoked": n}, domain.OutcomeAllowed)
	return nil
}
