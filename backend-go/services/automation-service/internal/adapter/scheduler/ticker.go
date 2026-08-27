// Package scheduler implements automation-service's in-process ticker loop
// — see specs/backend-go/services/automation-service.md §7. Every replica
// runs one (Ticker.Run, started as a goroutine from cmd/server/main.go);
// there is no separate scheduler service or external cron caller.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"
)

// Ticker claims due automations and dispatches each through the same
// usecase.RunNow interactor RunNow's gRPC handler uses — never a separate
// execution path, per automation-service.md §2/§6.
type Ticker struct {
	claimer   usecase.DueAutomationClaimer
	runNow    *usecase.RunNow
	interval  time.Duration
	batchSize int32
	logger    *slog.Logger

	done chan struct{}
}

// New builds a Ticker. logger defaults to slog.Default() if nil.
func New(claimer usecase.DueAutomationClaimer, runNow *usecase.RunNow, interval time.Duration, batchSize int32, logger *slog.Logger) *Ticker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Ticker{
		claimer:   claimer,
		runNow:    runNow,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger,
		done:      make(chan struct{}),
	}
}

// Run blocks, ticking every t.interval until ctx is cancelled — intended to
// be started as `go ticker.Run(ctx)` from cmd/server/main.go, using the
// same top-level shutdown context every other goroutine there watches.
func (t *Ticker) Run(ctx context.Context) {
	defer close(t.done)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.tick(ctx)
		}
	}
}

// Done closes once Run has returned — an in-flight tick finishes first, so
// cmd/server/main.go can wait on this during graceful shutdown instead of
// cutting a claim batch off mid-dispatch.
func (t *Ticker) Done() <-chan struct{} { return t.done }

func (t *Ticker) tick(ctx context.Context) {
	batch, err := t.claimer.ClaimDue(ctx, time.Now().UTC(), t.batchSize)
	if err != nil {
		t.logger.Error("scheduler: claim due automations failed", slog.Any("error", err))
		return
	}

	for _, automation := range batch.Automations() {
		t.dispatch(ctx, batch, automation)
	}

	if err := batch.Commit(ctx); err != nil {
		t.logger.Error("scheduler: commit claim batch failed", slog.Any("error", err))
	}
}

// dispatch runs one claimed automation through RunNow, then advances its
// next_run_at within the same still-open claim transaction. The advance
// happens unconditionally, even when RunNow itself errors — see
// usecase.ClaimedBatch's doc comment: RunNow already fails closed and
// records the run Failed (automation-service.md §8), so a retry of THIS
// occurrence would just hit the idempotency check and return the same
// Failed run; leaving next_run_at stuck instead would make the automation
// "due" forever without ever reaching its next real occurrence.
func (t *Ticker) dispatch(ctx context.Context, batch usecase.ClaimedBatch, automation domain.Automation) {
	dispatchCtx := tenant.WithTenantID(ctx, automation.TenantID)
	requestID := scheduledRequestID(automation.ID, automation.NextRunAt)

	if _, err := t.runNow.Execute(dispatchCtx, usecase.RunNowInput{
		AutomationID: automation.ID,
		RequestID:    requestID,
		Trigger:      domain.RunTriggerScheduled,
	}); err != nil {
		t.logger.Error("scheduler: dispatch failed",
			slog.String("automation_id", automation.ID), slog.Any("error", err))
	}

	rule, err := automation.RecurrenceRule()
	if err != nil {
		// Unreachable in practice — rrule was already validated at creation
		// time (domain.NewAutomation) — but fail safe (stop rescheduling)
		// rather than looping on a rule that can no longer be parsed.
		t.logger.Error("scheduler: rebuilding recurrence rule failed",
			slog.String("automation_id", automation.ID), slog.Any("error", err))
		if aerr := batch.Advance(ctx, automation.ID, time.Time{}, false); aerr != nil {
			t.logger.Error("scheduler: advance next_run_at failed",
				slog.String("automation_id", automation.ID), slog.Any("error", aerr))
		}
		return
	}

	next, hasNext := rule.NextOccurrenceAfter(automation.NextRunAt)
	if err := batch.Advance(ctx, automation.ID, next, hasNext); err != nil {
		t.logger.Error("scheduler: advance next_run_at failed",
			slog.String("automation_id", automation.ID), slog.Any("error", err))
	}
}

// scheduledRequestID derives a deterministic idempotency key from
// (automation_id, next_run_at) — automation-service.md §7/§8: ticks are
// at-least-once, so a retried claim of the same due occurrence (this
// replica after a crash, or another replica during a network partition)
// must land on the same request_id RunNow already dedupes on, not mint a
// fresh one that would double-dispatch.
func scheduledRequestID(automationID string, nextRunAt time.Time) string {
	return fmt.Sprintf("scheduled:%s:%d", automationID, nextRunAt.UnixNano())
}
