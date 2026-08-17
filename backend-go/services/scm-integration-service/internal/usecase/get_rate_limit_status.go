package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// GetRateLimitStatusInput mirrors GetRateLimitStatusRequest 1:1.
type GetRateLimitStatusInput struct {
	TenantID string
	Provider domain.ScmProvider
}

// GetRateLimitStatus resolves this tenant's per-provider credential,
// resolves the concrete provider adapter, and delegates — see §8: this is
// the client-facing surface of the proactive-throttling requirement.
type GetRateLimitStatus struct {
	credentials CredentialResolver
	providers   ProviderRegistry
}

func NewGetRateLimitStatus(credentials CredentialResolver, providers ProviderRegistry) *GetRateLimitStatus {
	return &GetRateLimitStatus{credentials: credentials, providers: providers}
}

func (uc *GetRateLimitStatus) Execute(ctx context.Context, in GetRateLimitStatusInput) (domain.RateLimitStatus, error) {
	if in.TenantID == "" {
		return domain.RateLimitStatus{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return domain.RateLimitStatus{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}

	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return domain.RateLimitStatus{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	status, err := provider.GetRateLimitStatus(ctx, cred)
	if err != nil {
		return domain.RateLimitStatus{}, apperrors.New(apperrors.KindInternal, "SCM_RATE_LIMIT_STATUS_FAILED", "failed to fetch rate limit status", err)
	}
	return status, nil
}
