package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestGetUsageToday_ReturnsRollupFromRepository(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)
	repo := &fakeUsageRepository{states: map[string]domain.QuotaState{
		"acc-1": {AccountID: "acc-1", Date: domain.DayKey(fixedNow), CostUSD: 4.20, RequestCount: 12},
	}}
	uc := NewGetUsageToday(repo, func() time.Time { return fixedNow })
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, GetUsageTodayInput{AccountID: "acc-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CostUSD != 4.20 || got.RequestCount != 12 {
		t.Errorf("expected rollup from repository, got %+v", got)
	}
}

func TestGetUsageToday_ZeroForUnknownAccount(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)
	repo := &fakeUsageRepository{states: map[string]domain.QuotaState{}}
	uc := NewGetUsageToday(repo, func() time.Time { return fixedNow })
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, GetUsageTodayInput{AccountID: "acc-unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CostUSD != 0 || got.RequestCount != 0 {
		t.Errorf("expected zero usage for unknown account, got %+v", got)
	}
}

func TestGetUsageToday_RequiresTenantContext(t *testing.T) {
	uc := NewGetUsageToday(&fakeUsageRepository{states: map[string]domain.QuotaState{}}, nil)
	_, err := uc.Execute(context.Background(), GetUsageTodayInput{AccountID: "acc-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
