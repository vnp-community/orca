package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// observeRun is the running step: look at the run the job follows.
func (w *PlanAutoWorker) observeRun(ctx context.Context, job domain.PlanAutoJob) {
	run, err := w.runs.Get(ctx, job.RunID)
	if isNotFoundKind(err) {
		w.finish(ctx, job, domain.PlanAutoSuperseded, planAutoReasonNewerRun) // the run was removed (retention) or never existed
		return
	}
	if err != nil {
		w.retryLater(ctx, job, err)
		return
	}
	switch run.Status {
	case domain.RunStatusSucceeded:
		w.reschedule(ctx, job, domain.PlanAutoCommitting, 0)
	case domain.RunStatusFailed:
		w.runFailed(ctx, job, derefStr(run.ErrorCode))
	default:
		if w.newerRunByPerson(ctx, job, run) {
			w.finish(ctx, job, domain.PlanAutoSuperseded, planAutoReasonNewerRun)
			return
		}
		w.reschedule(ctx, job, domain.PlanAutoRunning, w.policy.PollInterval)
	}
}

// runFailed decides between one more run and giving up. Only a transient code buys another run, and only while the job
// has attempts left: a code that would fail the same way (invalid output, missing configuration) must not cost twice.
func (w *PlanAutoWorker) runFailed(ctx context.Context, job domain.PlanAutoJob, code string) {
	if code == "" {
		code = domain.RunErrInternal
	}
	if domain.IsTransientPlanRunCode(code) && job.Attempt < job.MaxAttempts {
		job.Attempt++
		job.RunID, job.ReasonCode = "", code
		w.reschedule(ctx, job, domain.PlanAutoPending, w.policy.RetryDelay)
		return
	}
	w.finish(ctx, job, domain.PlanAutoGaveUp, code)
}

// newerRunByPerson reports a Plan run that a person started after the one the job follows: they regenerated, so the job
// has nothing left to say.
func (w *PlanAutoWorker) newerRunByPerson(ctx context.Context, job domain.PlanAutoJob, current domain.AnalysisRun) bool {
	recent, err := w.runs.ListRecent(ctx, job.RequestID, planAutoRecentRuns)
	if err != nil {
		return false
	}
	for _, run := range recent {
		if run.Kind != domain.RunKindPlan {
			continue
		}
		return run.ID != current.ID && !ownsRun(job, run) && run.StartedAt.After(current.StartedAt)
	}
	return false
}
