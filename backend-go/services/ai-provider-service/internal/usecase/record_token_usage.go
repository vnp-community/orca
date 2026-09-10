package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

type RecordTokenUsageInput struct {
	AccountID    string
	TokensUsed   int64
	RequestCount int64
	CostUSD      float64
}

// RecordTokenUsage upserts today's rollup and enforces quota — off the
// Resolve hot path per §8, called synchronously by whichever service
// dispatched the agent execution once it completes.
type RecordTokenUsage struct {
	accounts ProviderAccountRepository
	usage    UsageRepository
	outbox   OutboxEnqueuer
	now      func() time.Time
}

func NewRecordTokenUsage(accounts ProviderAccountRepository, usage UsageRepository, outbox OutboxEnqueuer, now func() time.Time) *RecordTokenUsage {
	if now == nil {
		now = time.Now
	}
	return &RecordTokenUsage{accounts: accounts, usage: usage, outbox: outbox, now: now}
}

func (uc *RecordTokenUsage) Execute(ctx context.Context, in RecordTokenUsageInput) (domain.QuotaState, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.QuotaState{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	account, err := uc.accounts.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return domain.QuotaState{}, err
	}

	today := domain.DayKey(uc.now())
	state, err := uc.usage.IncrementUsage(ctx, tenantID, in.AccountID, today, in.TokensUsed, in.RequestCount, in.CostUSD)
	if err != nil {
		return domain.QuotaState{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_USAGE_WRITE_FAILED", "failed to increment usage rollup", err)
	}

	if account.QuotaLimitDay == 0 { // 0 = unlimited
		return state, nil
	}

	ratio := float64(state.TokensUsed) / float64(account.QuotaLimitDay)
	switch {
	case ratio >= 1.0:
		detail := domain.HealthDetailQuotaExceeded
		if _, err := uc.accounts.UpdateStatus(ctx, UpdateStatusInput{
			TenantID: tenantID, AccountID: in.AccountID, Status: domain.AccountStatusError, HealthDetail: &detail,
		}); err != nil {
			return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_FLIP_FAILED", "usage recorded but failed to flip status on quota exceeded", err)
		}
		payload := map[string]any{"account_id": in.AccountID, "tokens_used": state.TokensUsed, "quota_limit_day": account.QuotaLimitDay}
		if err := uc.outbox.Enqueue(ctx, "ai_provider.usage.quota_exceeded", tenantID, payload); err != nil {
			return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_ALERT_FAILED", "usage recorded but failed to enqueue quota-exceeded alert", err)
		}
	case ratio >= 0.8:
		if account.QuotaWarningSentDate == nil || !account.QuotaWarningSentDate.Equal(today) {
			if err := uc.accounts.MarkQuotaWarningSent(ctx, tenantID, in.AccountID, today); err != nil {
				return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_WARNING_MARK_FAILED", "usage recorded but failed to mark warning sent", err)
			}
			payload := map[string]any{"account_id": in.AccountID, "tokens_used": state.TokensUsed, "quota_limit_day": account.QuotaLimitDay}
			if err := uc.outbox.Enqueue(ctx, "ai_provider.usage.quota_warning", tenantID, payload); err != nil {
				return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_WARNING_ALERT_FAILED", "usage recorded but failed to enqueue quota-warning alert", err)
			}
		}
	}
	return state, nil
}
