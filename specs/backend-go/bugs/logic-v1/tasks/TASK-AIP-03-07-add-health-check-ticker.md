# TASK-AIP-03-07: Add `health_check_ticker.go` and wire it into `main.go`

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/adapter/scheduler/health_check_ticker.go` (new)
**Depends on:** TASK-AIP-03-04, TASK-AIP-03-06
**Status:** `[ ]` TODO

---

## Context

`ReconcileProviderHealth.Execute` (`TASK-AIP-03-04`) needs something to
call it every 15 minutes, once per replica, per §8. Copy-adapted from
`automation-service`'s `scheduler.Ticker`
(`backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:19-60`)
— same `time.NewTicker`-driven `Run(ctx)` loop, no leader election needed
because `ClaimDue`'s `FOR UPDATE SKIP LOCKED` query is itself what
prevents double-firing across replicas.

## Changes to make

Create
`backend-go/services/ai-provider-service/internal/adapter/scheduler/health_check_ticker.go`:

```go
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
```

In `backend-go/services/ai-provider-service/cmd/server/main.go`, wire it
in (mirroring `automation-service/cmd/server/main.go:110-114`'s
background-goroutine convention):

```go
reconcileHealthUC := usecase.NewReconcileProviderHealth(repo, infraFleet, repo, nil)
recordTokenUsageUC := usecase.NewRecordTokenUsage(repo, repo, repo, nil)
healthCheckTicker := aiproviderscheduler.New(reconcileHealthUC, 15*time.Minute, 50, logger)
go healthCheckTicker.Run(ctx)
```
(`repo` satisfies `DueHealthCheckClaimer`/`OutboxEnqueuer`/
`UsageRepository` per `TASK-AIP-03-06`; import the new scheduler package
as `aiproviderscheduler` alongside the existing `aiprovidergrpc`/
`aiprovidergrpcclient`/`aiproviderpostgres` aliases.)

Also wire `recordTokenUsageUC` into `aiprovidergrpc.New(...)`'s
constructor call and `Server` struct — see `TASK-AIP-03-08` for the full
gRPC surface.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/ai-provider-service/internal/adapter/scheduler/...
```

Add `health_check_ticker_test.go`: a fake `reconcile` records call
timestamps; assert `Run` invokes it roughly every `interval` and stops
cleanly when `ctx` is cancelled (use a short interval, e.g. 10ms, and
`ctx`+`time.After` in the test to keep it fast).
