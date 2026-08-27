package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// RevokeAuthInput mirrors RevokeAuthRequest 1:1.
type RevokeAuthInput struct {
	TenantID string
	Provider domain.Provider
}

// RevokeAuth backs credentials.revoke for jira/linear — new to this service
// (unlike scm-integration-service, which already had a RevokeAuth usecase to
// reuse). Disconnects a provider by revoking its stored credential via
// CredentialRevoker.RevokeByOwner (category
// CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH, owner_id = provider name — the
// same provider-name-only convention SetIntegrationCredential/
// GetIntegrationCredentialStatus use, distinct from the per-user Connect
// flow's "<userID>:<provider>" owner_id).
type RevokeAuth struct {
	revoker CredentialRevoker
}

func NewRevokeAuth(revoker CredentialRevoker) *RevokeAuth {
	return &RevokeAuth{revoker: revoker}
}

func (uc *RevokeAuth) Execute(ctx context.Context, in RevokeAuthInput) error {
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_NO_TENANT", "tenant_id is required", nil)
	}
	if err := uc.revoker.RevokeByOwner(ctx, in.TenantID, in.Provider); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_REVOKE_FAILED", "failed to revoke credential", err)
	}
	return nil
}
