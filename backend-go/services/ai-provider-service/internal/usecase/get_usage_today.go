package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// GetUsageTodayInput mirrors the gRPC GetUsageTodayRequest.
type GetUsageTodayInput struct {
	AccountID string
}

// GetUsageToday reads today's quota/spend rollup for an account — a read of
// the aggregate rollup only, never raw usage events (ai_provider.usage,
// not usage-service's per-session data — see ai-provider-service.md §2's
// bounded-context distinction). Off the Resolve hot path entirely; this is
// its own RPC.
type GetUsageToday struct {
	repo UsageRepository
	now  func() time.Time
}

func NewGetUsageToday(repo UsageRepository, now func() time.Time) *GetUsageToday {
	if now == nil {
		now = time.Now
	}
	return &GetUsageToday{repo: repo, now: now}
}

func (uc *GetUsageToday) Execute(ctx context.Context, in GetUsageTodayInput) (domain.QuotaState, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.QuotaState{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	today := domain.DayKey(uc.now())
	state, err := uc.repo.GetToday(ctx, tenantID, in.AccountID, today)
	if err != nil {
		return domain.QuotaState{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_USAGE_FETCH_FAILED", "failed to fetch today's usage rollup", err)
	}
	return state, nil
}
