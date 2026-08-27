package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

type GetDailyUsageInput struct {
	UserID   string
	Provider domain.Provider
	Day      int64 // unix seconds, truncated to day by domain.DayKey
}

type GetDailyUsage struct {
	repo Repository
}

func NewGetDailyUsage(repo Repository) *GetDailyUsage {
	return &GetDailyUsage{repo: repo}
}

func (uc *GetDailyUsage) Execute(ctx context.Context, in GetDailyUsageInput) (domain.DailyUsageRollup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DailyUsageRollup{}, apperrors.New(apperrors.KindUnauthenticated, "USAGE_NO_TENANT", "no tenant in request context", err)
	}

	day := domain.DayKey(time.Unix(in.Day, 0))
	rollup, err := uc.repo.GetDailyRollup(ctx, tenantID, in.UserID, in.Provider, day)
	if err != nil {
		return domain.DailyUsageRollup{}, apperrors.New(apperrors.KindInternal, "USAGE_ROLLUP_FETCH_FAILED", "failed to fetch daily rollup", err)
	}
	return rollup, nil
}
