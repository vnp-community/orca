package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// fakeRepository is an in-memory Repository — the "test against fakes, not
// a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeRepository struct {
	saved   []domain.UsageSession
	events  []domain.OutboxEvent
	saveErr error
}

func (f *fakeRepository) SaveSession(ctx context.Context, s domain.UsageSession, event domain.OutboxEvent) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, s)
	f.events = append(f.events, event)
	return nil
}

func (f *fakeRepository) GetDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) (domain.DailyUsageRollup, error) {
	rollup := domain.DailyUsageRollup{TenantID: tenantID, UserID: userID, Provider: provider, Day: day}
	for _, s := range f.saved {
		if s.TenantID == tenantID && s.UserID == userID && s.Provider == provider && domain.DayKey(s.StartedAt).Equal(day) {
			rollup = rollup.ApplySession(s)
		}
	}
	return rollup, nil
}

func (f *fakeRepository) ListSessions(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.UsageSession, string, error) {
	var out []domain.UsageSession
	for _, s := range f.saved {
		if s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	return out, "", nil
}

func (f *fakeRepository) RecomputeDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) error {
	return nil
}

func withIdentity(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}

func TestRecordUsageSession_RequiresTenantContext(t *testing.T) {
	uc := NewRecordUsageSession(&fakeRepository{})
	_, err := uc.Execute(context.Background(), RecordUsageSessionInput{
		ID: "s1", Provider: domain.ProviderClaude, RequestID: "req-1",
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRecordUsageSession_SavesAndEnqueuesOutboxEvent(t *testing.T) {
	repo := &fakeRepository{}
	uc := NewRecordUsageSession(repo)

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, RecordUsageSessionInput{
		ID:           "s1",
		Provider:     domain.ProviderClaude,
		WorktreeID:   "wt-1",
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.05,
		StartedAt:    time.Now().Unix(),
		RequestID:    "req-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.UserID != "user-1" {
		t.Errorf("expected tenant/user to come from context, got tenant=%s user=%s", got.TenantID, got.UserID)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved session, got %d", len(repo.saved))
	}
	if len(repo.events) != 1 {
		t.Fatalf("expected 1 outbox event enqueued in the same call as the session write, got %d", len(repo.events))
	}
	if repo.events[0].Subject != SessionRecordedSubject || repo.events[0].ID == "" {
		t.Errorf("unexpected outbox event: %+v", repo.events[0])
	}
}

func TestRecordUsageSession_RepositoryFailurePropagates(t *testing.T) {
	repo := &fakeRepository{saveErr: errors.New("db unavailable")}
	uc := NewRecordUsageSession(repo)

	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, RecordUsageSessionInput{
		ID: "s1", Provider: domain.ProviderClaude, RequestID: "req-1", StartedAt: time.Now().Unix(),
	})
	if err == nil {
		t.Fatal("expected error to propagate from repository failure")
	}
}
