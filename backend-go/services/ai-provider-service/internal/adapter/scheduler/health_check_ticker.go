// Package scheduler implements ai-provider-service's in-process
// health-check ticker — see specs/backend-go/services/ai-provider-service.md
// §8. Every replica runs one (HealthCheckTicker.Run, started as a goroutine
// from cmd/server/main.go); no separate scheduler service or external cron
// caller, same posture as automation-service's Ticker.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

// HealthCheckTicker calls ReconcileProviderHealth.Execute every interval —
// no leader election needed: the claim query itself (SELECT ... FOR UPDATE
// SKIP LOCKED) is what prevents double-firing across replicas.
type HealthCheckTicker struct {
	reconcile *usecase.ReconcileProviderHealth
	interval  time.Duration
	batchSize int32
	logger    *slog.Logger
	done      chan struct{}
}

// New builds a HealthCheckTicker. logger defaults to slog.Default() if nil;
// interval defaults to 15 minutes per §8 if <= 0.
func New(reconcile *usecase.ReconcileProviderHealth, interval time.Duration, batchSize int32, logger *slog.Logger) *HealthCheckTicker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthCheckTicker{reconcile: reconcile, interval: interval, batchSize: batchSize, logger: logger, done: make(chan struct{})}
}

// Run blocks, ticking every t.interval until ctx is cancelled — intended to
// be started as `go ticker.Run(ctx)` from cmd/server/main.go.
func (t *HealthCheckTicker) Run(ctx context.Context) {
	defer close(t.done)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.reconcile.Execute(ctx, t.batchSize); err != nil {
				t.logger.Error("health-check ticker: reconcile failed", slog.Any("error", err))
			}
		}
	}
}

// Done closes once Run has returned — an in-flight tick finishes first.
func (t *HealthCheckTicker) Done() <-chan struct{} { return t.done }
