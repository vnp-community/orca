package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// RevokeAuthInput mirrors RevokeAuthRequest 1:1.
type RevokeAuthInput struct {
	TenantID string
	Provider domain.ScmProvider
}

// RevokeAuth disconnects a provider by revoking its stored credential via
// CredentialRevoker.RevokeByOwner (category CREDENTIAL_CATEGORY_SCM_OAUTH,
// owner_id = provider name).
//
// PRIOR GAP, NOW CLOSED: this usecase used to be a hard-coded
// SCM_REVOKE_REQUIRES_BROKER_RPC error — credential-broker-service only
// exposed RevokeCredential(credential_id), and this service (like
// GetAuthStatus/CredentialResolver) only ever holds (tenant_id, provider),
// never an opaque credential_id. credential-broker-service now exposes
// RevokeCredentialByOwner for exactly this caller shape (mirroring
// ResolveCredentialByOwner on the read side) — see this service's README
// "Known gaps" for the resolved entry.
type RevokeAuth struct {
	revoker CredentialRevoker
}

func NewRevokeAuth(revoker CredentialRevoker) *RevokeAuth {
	return &RevokeAuth{revoker: revoker}
}

func (uc *RevokeAuth) Execute(ctx context.Context, in RevokeAuthInput) error {
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if err := uc.revoker.RevokeByOwner(ctx, in.TenantID, in.Provider); err != nil {
		return apperrors.New(apperrors.KindInternal, "SCM_REVOKE_FAILED", "failed to revoke credential", err)
	}
	return nil
}
