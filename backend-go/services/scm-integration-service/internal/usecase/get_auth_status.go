package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// GetAuthStatusInput mirrors GetAuthStatusRequest 1:1.
type GetAuthStatusInput struct {
	TenantID string
	Provider domain.ScmProvider
}

// GetAuthStatus reports whether this tenant currently has a usable
// credential for provider. There is no dedicated "is this connected"
// broker RPC keyed by (tenant, category, owner) — only resolve-by-id
// metadata lookups, which this service can't use (it never holds an opaque
// credential_id, see credentialbroker adapter's package doc comment) — so
// this simply attempts CredentialResolver.Resolve and treats success with a
// non-empty token as "connected". A resolution failure (not-found or
// otherwise) is reported as connected=false, not surfaced as an RPC error:
// "not connected yet" is an expected, common state for this RPC to answer,
// not a failure of the check itself.
type GetAuthStatus struct {
	credentials CredentialResolver
}

func NewGetAuthStatus(credentials CredentialResolver) *GetAuthStatus {
	return &GetAuthStatus{credentials: credentials}
}

func (uc *GetAuthStatus) Execute(ctx context.Context, in GetAuthStatusInput) (bool, error) {
	if in.TenantID == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return false, nil
	}
	return cred.Token != "", nil
}
