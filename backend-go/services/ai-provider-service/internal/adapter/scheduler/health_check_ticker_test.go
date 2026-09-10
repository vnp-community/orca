package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

// TestHealthCheckTicker_TicksAndStopsOnCancel drives HealthCheckTicker
// against a real *usecase.ReconcileProviderHealth wired to fakes local to
// this test file — asserts the externally-observable behavior: it invokes
// the claimer roughly every interval, and Run returns (closing Done())
// shortly after ctx is cancelled.
func TestHealthCheckTicker_TicksAndStopsOnCancel(t *testing.T) {
	claimer := &countingClaimer{}
	reconcile := usecase.NewReconcileProviderHealth(claimer, noopInfraFleetClient{}, noopOutboxEnqueuer{}, time.Now)

	ticker := New(reconcile, 10*time.Millisecond, 5, nil)
	ctx, cancel := context.WithCancel(context.Background())

	go ticker.Run(ctx)

	// Let a few ticks fire.
	time.Sleep(55 * time.Millisecond)
	cancel()

	select {
	case <-ticker.Done():
	case <-time.After(time.Second):
		t.Fatal("expected Run to return and close Done() shortly after ctx is cancelled")
	}

	if claimer.calls() < 2 {
		t.Errorf("expected at least 2 ticks to have fired in ~55ms at a 10ms interval, got %d", claimer.calls())
	}
}

// countingClaimer implements usecase.DueHealthCheckClaimer, returning an
// empty batch each time and counting how many times it was claimed.
type countingClaimer struct {
	n int
}

func (c *countingClaimer) calls() int { return c.n }

func (c *countingClaimer) ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (usecase.ClaimedHealthCheckBatch, error) {
	c.n++
	return emptyBatch{}, nil
}

// emptyBatch implements usecase.ClaimedHealthCheckBatch with no accounts —
// this test only cares that the claimer got invoked on schedule, not about
// dispatch behavior (covered by reconcile_provider_health_test.go).
type emptyBatch struct{}

func (emptyBatch) Accounts() []domain.ProviderAccount { return nil }
func (emptyBatch) RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error {
	return nil
}
func (emptyBatch) Commit(ctx context.Context) error   { return nil }
func (emptyBatch) Rollback(ctx context.Context) error { return nil }

type noopInfraFleetClient struct{}

func (noopInfraFleetClient) Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error) {
	return map[string]any{"success": true}, nil
}

type noopOutboxEnqueuer struct{}

func (noopOutboxEnqueuer) Enqueue(ctx context.Context, subject string, tenantID string, payload map[string]any) error {
	return nil
}
