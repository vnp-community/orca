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
	plan      AnalysisWorker
	proposal  AnalysisWorker
	observer  EngineObserver
	requests  AnalysisRequestGetter
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

// WithPlanWorker adds the worker for kind=plan runs; they share the lease, heartbeat and recovery of the others.
func (r *AnalysisRunner) WithPlanWorker(w AnalysisWorker) *AnalysisRunner {
	r.plan = w
	return r
}

// WithProposalWorker adds the worker for agent_proposal plan runs (engine "openspec"). Without it such a run fails at once
// with a clear code instead of being written by the ai.complete plan worker, which would silently ignore the engine.
func (r *AnalysisRunner) WithProposalWorker(w AnalysisWorker) *AnalysisRunner {
	r.proposal = w
	return r
}

// AnalysisRequestGetter is the part of the Request store the runner reads: who reported it.
type AnalysisRequestGetter interface {
	Get(ctx context.Context, id string) (domain.Request, error)
}

// WithRequests lets analysis runs started by someone other than the reporter find the dev server, which is bound to the reporter.
func (r *AnalysisRunner) WithRequests(q AnalysisRequestGetter) *AnalysisRunner {
	r.requests = q
	return r
}

// reporterIdentity makes a run started by an approver or admin look up its dev server as the reporter; the run stays
// attributed to the caller. Plan runs do the same since CR-REQ-045: an approver revises a Plan on the reporter's dev
// server, and the workers only use the identity to route to that server. A failed lookup keeps the caller's identity,
// so the run then fails with the same no-connection error it would have had.
func (r *AnalysisRunner) reporterIdentity(ctx context.Context, run domain.AnalysisRun) context.Context {
	if r.requests == nil || run.ActorID == "" {
		return ctx
	}
	req, err := r.requests.Get(ctx, run.RequestID)
	if err != nil || req.ReporterID == "" || req.ReporterID == run.ActorID {
		return ctx
	}
	return tenant.WithUserID(ctx, req.ReporterID)
}

// WithObserver reports how each run ended, for the engine metrics.
func (r *AnalysisRunner) WithObserver(o EngineObserver) *AnalysisRunner {
	r.observer = o
	return r
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
	if run.Kind == domain.RunKindPlan && run.Mode == domain.AnalysisModeAgentProposal {
		if r.proposal != nil {
			return r.proposal
		}
		return engineUnavailableWorker{}
	}
	if run.Kind == domain.RunKindPlan && r.plan != nil {
		return r.plan
	}
	if run.Mode == domain.AnalysisModeAgentReadonly {
		return r.agent
	}
	return r.complete
}

func (r *AnalysisRunner) execute(run domain.AnalysisRun) {
	ctx, cancel := context.WithCancel(r.runCtx(run))
	defer cancel()
	ctx = r.reporterIdentity(ctx, run)
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
	if err == nil {
		observerOrNoop(r.observer).RunFinished(engineLabel(run), string(run.Kind), "succeeded")
		return
	}
	if errors.Is(err, ErrLeaseLost) {
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
	observerOrNoop(r.observer).RunFinished(engineLabel(run), string(run.Kind), "failed")
}

// failRun records the failure and drops the draft in one transaction, so a failed run never leaves a draft Solution.
func (r *AnalysisRunner) failRun(ctx context.Context, run domain.AnalysisRun, f *AnalysisFailure) {
	err := r.tx.InTx(ctx, func(ctx context.Context) error {
		failed := run
		failed.Attempt, failed.Enforcement, failed.RepoCheck = max(f.Attempt, run.Attempt), f.Enforcement, f.RepoCheck
		if f.Stats != nil {
			failed.Stats = f.Stats
		}
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
		return deleteDraftIfAny(ctx, r.solutions, run)
	})
	if err != nil && !errors.Is(err, ErrLeaseLost) {
		slog.Warn("could not record analysis failure; the sweeper will", slog.String("run_id", run.ID), slog.Any("error", err))
	}
}

// deleteDraftIfAny drops the draft Solution of a failed run; plan runs never had one.
func deleteDraftIfAny(ctx context.Context, store SolutionStore, run domain.AnalysisRun) error {
	if run.SolutionID == "" {
		return nil
	}
	return store.DeleteDraft(ctx, run.SolutionID)
}

// RunErrEngineUnavailable is the code of a run whose engine has no worker on this server.
const RunErrEngineUnavailable = "REQUEST_ENGINE_UNAVAILABLE"

type engineUnavailableWorker struct{}

func (engineUnavailableWorker) Execute(_ context.Context, run domain.AnalysisRun) error {
	return newFailure(run, RunErrEngineUnavailable, "this server has no worker for the run's engine")
}
