package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// rateLimitCacheFreshness bounds how old a cached rate-limit snapshot may
// be before GetRateLimitStatus falls back to a live provider call — short
// enough that clients still see a near-real-time number, long enough that
// a client polling this RPC repeatedly doesn't burn quota just checking
// quota (§8's stated rationale for this table existing at all).
const rateLimitCacheFreshness = 60 * time.Second

// GetRateLimitStatusInput mirrors GetRateLimitStatusRequest 1:1.
type GetRateLimitStatusInput struct {
	TenantID string
	Provider domain.ScmProvider
}

// GetRateLimitStatus resolves this tenant's per-provider credential,
// resolves the concrete provider adapter, and delegates — see §8: this is
// the client-facing surface of the proactive-throttling requirement. Reads
// through RateLimitCache first (a fresh-enough cached snapshot skips the
// live provider call entirely) and writes a live result back into it for
// next time.
type GetRateLimitStatus struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	cache       RateLimitCache // nil is valid — see NewGetRateLimitStatus.
}

// NewGetRateLimitStatus wires GetRateLimitStatus. cache may be nil (no
// database configured — see cmd/server/main.go), in which case every call
// goes straight to the live provider, same as before this cache existed.
func NewGetRateLimitStatus(credentials CredentialResolver, providers ProviderRegistry, cache RateLimitCache) *GetRateLimitStatus {
	return &GetRateLimitStatus{credentials: credentials, providers: providers, cache: cache}
}

func (uc *GetRateLimitStatus) Execute(ctx context.Context, in GetRateLimitStatusInput) (domain.RateLimitStatus, error) {
	if in.TenantID == "" {
		return domain.RateLimitStatus{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}

	if uc.cache != nil {
		// A cache read error is treated the same as a miss — this cache is
		// a local convenience (§5: "not a source of truth"), never allowed
		// to turn a working RPC into a failure.
		if cached, ok, err := uc.cache.Get(ctx, in.TenantID, in.Provider, rateLimitCacheFreshness); err == nil && ok {
			return cached, nil
		}
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

	if uc.cache != nil {
		// Best-effort: a cache write failure must not fail an RPC that
		// already has a real, live answer to return.
		_ = uc.cache.Set(ctx, in.TenantID, in.Provider, status)
	}

	return status, nil
}
