package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func seedQuotaAccount(t *testing.T, repo *fakeAccountRepository, id string, quotaLimitDay int) {
	t.Helper()
	now := time.Now()
	acc, err := domain.NewProviderAccount(id, "tenant-1", domain.ProviderTypeAnthropic, domain.AccountStatusActive,
		"cred-"+id, domain.ScopeServer, "", "", "dev-1", "", "", "", quotaLimitDay, nil, false, nil, "", nil, now, now)
	if err != nil {
		t.Fatalf("building account: %v", err)
	}
	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatalf("seeding account: %v", err)
	}
}

func TestRecordTokenUsage_UnlimitedQuotaNeverFlips(t *testing.T) {
	repo := newFakeAccountRepository()
	seedQuotaAccount(t, repo, "acc-1", 0) // 0 = unlimited
	usage := &fakeUsageRepository{}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewRecordTokenUsage(repo, usage, outboxFake, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, RecordTokenUsageInput{AccountID: "acc-1", TokensUsed: 10_000_000, RequestCount: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.accounts["acc-1"].Status != domain.AccountStatusActive {
		t.Errorf("expected status to remain active for an unlimited-quota account, got %q", repo.accounts["acc-1"].Status)
	}
	if len(outboxFake.enqueued) != 0 {
		t.Errorf("expected no outbox event for an unlimited-quota account, got %d", len(outboxFake.enqueued))
	}
}

func TestRecordTokenUsage_80PercentWarningOnce(t *testing.T) {
	repo := newFakeAccountRepository()
	seedQuotaAccount(t, repo, "acc-1", 1000)
	usage := &fakeUsageRepository{}
	outboxFake := &fakeOutboxEnqueuer{}
	day1 := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	now := day1
	uc := NewRecordTokenUsage(repo, usage, outboxFake, func() time.Time { return now })
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	// First call: 850/1000 = 85% >= 80% -> warning fires.
	if _, err := uc.Execute(ctx, RecordTokenUsageInput{AccountID: "acc-1", TokensUsed: 850, RequestCount: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Second call same day: still >= 80%, must NOT fire a second warning.
	if _, err := uc.Execute(ctx, RecordTokenUsageInput{AccountID: "acc-1", TokensUsed: 10, RequestCount: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outboxFake.enqueued) != 1 {
		t.Fatalf("expected exactly one quota_warning event across two same-day calls, got %d", len(outboxFake.enqueued))
	}

	// Third call, next day (mocked now): a second warning is allowed.
	now = day1.Add(24 * time.Hour)
	if _, err := uc.Execute(ctx, RecordTokenUsageInput{AccountID: "acc-1", TokensUsed: 900, RequestCount: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outboxFake.enqueued) != 2 {
		t.Fatalf("expected a second quota_warning event on a new UTC day, got %d", len(outboxFake.enqueued))
	}
}

func TestRecordTokenUsage_QuotaExceededFlipsStatusAndAlerts(t *testing.T) {
	repo := newFakeAccountRepository()
	seedQuotaAccount(t, repo, "acc-1", 1000)
	usage := &fakeUsageRepository{}
	outboxFake := &fakeOutboxEnqueuer{}
	uc := NewRecordTokenUsage(repo, usage, outboxFake, nil)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, RecordTokenUsageInput{AccountID: "acc-1", TokensUsed: 1000, RequestCount: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastUpdateStatusInput.Status != domain.AccountStatusError {
		t.Errorf("expected UpdateStatus to be called with AccountStatusError, got %q", repo.lastUpdateStatusInput.Status)
	}
	if repo.lastUpdateStatusInput.HealthDetail == nil || *repo.lastUpdateStatusInput.HealthDetail != domain.HealthDetailQuotaExceeded {
		t.Errorf("expected UpdateStatus to be called with HealthDetail=quota_exceeded, got %v", repo.lastUpdateStatusInput.HealthDetail)
	}
	if len(outboxFake.enqueued) != 1 || outboxFake.enqueued[0].subject != "ai_provider.usage.quota_exceeded" {
		t.Fatalf("expected one quota_exceeded outbox event, got %+v", outboxFake.enqueued)
	}
	refetched, err := repo.Get(ctx, "tenant-1", "acc-1")
	if err != nil {
		t.Fatalf("unexpected error refetching account: %v", err)
	}
	if refetched.Resolvable() {
		t.Error("expected the refetched account to no longer be Resolvable() after quota exceeded")
	}
}
