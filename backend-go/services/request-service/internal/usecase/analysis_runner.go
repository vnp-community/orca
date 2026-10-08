package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AnalysisWorker does the slow part of one run (the AI or agent call) and commits the result itself.
// It returns *AnalysisFailure to fail the run, ErrLeaseLost to stop silently, or nil after a commit.
type AnalysisWorker interface {
	Execute(ctx context.Context, run domain.AnalysisRun) error
}

// AnalysisRunner owns the background half of analysis (decision D3): the RPC has already returned, so a worker runs
// under its own context with a renewed lease, and a sweeper fails runs whose instance died.
type AnalysisRunner struct {
	runs      AnalysisRunStore
	solutions SolutionStore
	tx        TxRunner
	complete  AnalysisWorker
	agent     AnalysisWorker
	settings  AnalysisSettings
	owner     string

	stopCtx context.Context
	stop    context.CancelFunc
	wg      sync.WaitGroup
}

var _ AnalysisSpawner = (*AnalysisRunner)(nil)

func NewAnalysisRunner(runs AnalysisRunStore, solutions SolutionStore, tx TxRunner, complete, agent AnalysisWorker, settings AnalysisSettings, owner string) *AnalysisRunner {
	stopCtx, stop := context.WithCancel(context.Background())
	return &AnalysisRunner{runs: runs, solutions: solutions, tx: tx, complete: complete, agent: agent, settings: settings.withDefaults(), owner: owner, stopCtx: stopCtx, stop: stop}
}

// Close stops workers and the sweeper and waits for them. Runs cut short stay running and are failed by the next sweep.
func (r *AnalysisRunner) Close() {
	r.stop()
	r.wg.Wait()
}

func (r *AnalysisRunner) Spawn(run domain.AnalysisRun) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.execute(run)
	}()
}

// runCtx rebuilds the identity of the original caller; the RPC context is long gone and must not cancel the work.
func (r *AnalysisRunner) runCtx(run domain.AnalysisRun) context.Context {
	ctx := tenant.WithTenantID(r.stopCtx, run.TenantID)
	if run.ActorID != "" {
		ctx = tenant.WithUserID(ctx, run.ActorID)
	}
	return ctx
}

func (r *AnalysisRunner) worker(run domain.AnalysisRun) AnalysisWorker {
	if run.Mode == domain.AnalysisModeAgentReadonly {
		return r.agent
	}
	return r.complete
}

func (r *AnalysisRunner) execute(run domain.AnalysisRun) {
	ctx, cancel := context.WithCancel(r.runCtx(run))
	defer cancel()
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		t := time.NewTicker(r.settings.Heartbeat)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ok, err := r.runs.RenewLease(ctx, run.ID, r.owner, r.settings.LeaseTTL)
				if err == nil && !ok {
					cancel() // another instance owns the run now
					return
				}
			}
		}
	}()
	err := r.worker(run).Execute(ctx, run)
	cancel()
	<-hbDone
	if err == nil || errors.Is(err, ErrLeaseLost) {
		return
	}
	if r.stopCtx.Err() != nil {
		return // shutting down: the lease expires and the sweeper decides
	}
	fail, ok := asFailure(err)
	if !ok {
		slog.Error("analysis run failed unexpectedly", slog.String("run_id", run.ID), slog.Any("error", err))
		fail = newFailure(run, domain.RunErrInternal, "internal error while running the analysis")
	}
	r.failRun(r.runCtx(run), run, fail)
}

// failRun records the failure and drops the draft in one transaction, so a failed run never leaves a draft Solution.
func (r *AnalysisRunner) failRun(ctx context.Context, run domain.AnalysisRun, f *AnalysisFailure) {
	err := r.tx.InTx(ctx, func(ctx context.Context) error {
		failed := run
		failed.Attempt, failed.Enforcement, failed.RepoCheck = max(f.Attempt, run.Attempt), f.Enforcement, f.RepoCheck
		failed.Fail(f.Code, f.Message, time.Now().UTC())
		if f.Raw != "" {
			raw := RedactRaw(f.Raw)
			failed.RawOutput = &raw
		}
		ok, err := r.runs.FinishOwned(ctx, failed, r.owner)
		if err != nil {
			return err
		}
		if !ok {
			return ErrLeaseLost // someone else finished or took over the run; leave the draft to them
		}
		return r.solutions.DeleteDraft(ctx, run.SolutionID)
	})
	if err != nil && !errors.Is(err, ErrLeaseLost) {
		slog.Warn("could not record analysis failure; the sweeper will", slog.String("run_id", run.ID), slog.Any("error", err))
	}
}
