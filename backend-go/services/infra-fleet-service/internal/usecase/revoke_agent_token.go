package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RevokeAgentToken revokes a token and closes any live session
// authenticated with it — immediate-effect, no deploy required (see
// SOL-AWS-01's "resolve on every dial" guarantee for the relay-websocket
// case, and SOL-AWS-03's LiveSessionCloser for direct-websocket).
type RevokeAgentToken struct {
	repo     AgentTokenRepository
	sessions LiveSessionCloser
}

func NewRevokeAgentToken(repo AgentTokenRepository, sessions LiveSessionCloser) *RevokeAgentToken {
	return &RevokeAgentToken{repo: repo, sessions: sessions}
}

func (uc *RevokeAgentToken) Execute(ctx context.Context, devServerID, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	tok, err := uc.repo.Revoke(ctx, tenantID, id)
	if err != nil {
		return err
	}
	_, err = uc.sessions.CloseSessionsForDevServerToken(ctx, devServerID, tok.ID)
	return err
}
