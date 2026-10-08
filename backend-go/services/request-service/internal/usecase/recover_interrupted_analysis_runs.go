package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RecoverInterruptedAnalysisRuns fails runs whose instance died. It does not resume them: the agent may still be
// running on the dev server, and re-running would double the cost; the user simply triggers a new run.
type RecoverInterruptedAnalysisRuns struct {
	runs      AnalysisRunStore
	solutions SolutionStore
	tx        TxRunner
	settings  AnalysisSettings
	owner     string
	batch     int
}

func NewRecoverInterruptedAnalysisRuns(runs AnalysisRunStore, solutions SolutionStore, tx TxRunner, settings AnalysisSettings, owner string) *RecoverInterruptedAnalysisRuns {
	return &RecoverInterruptedAnalysisRuns{runs: runs, solutions: solutions, tx: tx, settings: settings.withDefaults(), owner: owner, batch: 20}
}

// RunOnce claims expired runs and fails each in its own tenant transaction; it returns how many it failed.
func (uc *RecoverInterruptedAnalysisRuns) RunOnce(ctx context.Context) (int, error) {
	claimed, err := uc.runs.ClaimExpired(ctx, uc.owner, uc.settings.LeaseTTL, uc.batch)
	if err != nil {
		return 0, err
	}
	failed := 0
	for _, run := range claimed {
		tctx := tenant.WithTenantID(ctx, run.TenantID)
		err := uc.tx.InTx(tctx, func(tctx context.Context) error {
			run.Fail(domain.RunErrInterrupted, "the instance running this analysis stopped before it finished", time.Now().UTC())
			ok, err := uc.runs.FinishOwned(tctx, run, uc.owner)
			if err != nil || !ok {
				return err
			}
			return uc.solutions.DeleteDraft(tctx, run.SolutionID)
		})
		if err != nil {
			slog.WarnContext(ctx, "could not fail interrupted analysis run", slog.String("run_id", run.ID), slog.Any("error", err))
			continue
		}
		failed++
	}
	return failed, nil
}

// RunRecoveryLoop sweeps until ctx ends; the first pass runs at once so a restart cleans up its predecessor's runs.
func (uc *RecoverInterruptedAnalysisRuns) RunRecoveryLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = uc.settings.RecoveryInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := uc.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.WarnContext(ctx, "analysis recovery sweep failed", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
