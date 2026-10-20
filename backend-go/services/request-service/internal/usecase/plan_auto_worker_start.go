package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// startRun is the pending step: ask for the run with this attempt's key. A key that already has a run hands that run
// back, so a repeated step (lost lease, failed save) and a person pressing Generate at the same time both end up
// following one run.
func (w *PlanAutoWorker) startRun(ctx context.Context, job domain.PlanAutoJob, req domain.Request) {
	if superseded := w.finishedRunByPerson(ctx, job); superseded {
		w.finish(ctx, job, domain.PlanAutoSuperseded, planAutoReasonNewerRun)
		return
	}
	// The deployment switch and the project switch are read again: a job may have waited for a busy project.
	if job.Origin == domain.PlanAutoOriginEvent {
		on, err := w.sw.Enabled(ctx, req.ProjectID)
		if err != nil {
			w.retryLater(ctx, job, err)
			return
		}
		if !on {
			w.finish(ctx, job, domain.PlanAutoSkipped, domain.PlanAutoCodeDisabled)
			return
		}
	}
	key, err := domain.PlanAutoKey(job.Anchor, job.Attempt)
	if err != nil {
		w.finish(ctx, job, domain.PlanAutoSkipped, errorCode(err))
		return
	}
	res, err := w.start.ExecuteForSystem(ctx, GeneratePlanInput{RequestID: job.RequestID, IdempotencyKey: key})
	switch {
	case err == nil:
		w.started(ctx, job, res)
	case errorHasCode(err, planAutoCodeBusy):
		w.waitForRoom(ctx, job)
	case permanentKind(err):
		w.finish(ctx, job, domain.PlanAutoSkipped, errorCode(err))
	default:
		w.retryLater(ctx, job, err)
	}
}

func (w *PlanAutoWorker) started(ctx context.Context, job domain.PlanAutoJob, res GeneratePlanResult) {
	job.RunID = res.RunID
	switch res.Status {
	case domain.RunStatusFailed:
		// The key matched a run that already failed (a repeated step after the failure): treat it as the failure it is.
		w.runFailed(ctx, job, res.ErrorCode)
	case domain.RunStatusSucceeded:
		job.ReasonCode = ""
		w.reschedule(ctx, job, domain.PlanAutoRunning, 0) // adopted a finished run: the next pass moves on to commit it
	default:
		job.ReasonCode = "" // an earlier busy or transient reason no longer describes the job
		w.reschedule(ctx, job, domain.PlanAutoRunning, w.policy.PollInterval)
	}
}

// waitForRoom parks the job while the project's agent runs are all busy; being busy is not a failure.
func (w *PlanAutoWorker) waitForRoom(ctx context.Context, job domain.PlanAutoJob) {
	if w.now().Sub(job.CreatedAt) > w.policy.StartWindow {
		w.finish(ctx, job, domain.PlanAutoGaveUp, planAutoCodeBusy)
		return
	}
	slog.InfoContext(ctx, "plan auto-generation waits for a free agent", slog.String("request_id", job.RequestID))
	job.ReasonCode = planAutoCodeBusy
	w.reschedule(ctx, job, domain.PlanAutoPending, w.policy.BusyDelay)
}

// finishedRunByPerson reports a Plan run that someone else started and already finished since the job began: the
// person did what the job was about to do. A run that is still going is adopted instead, through the run store.
func (w *PlanAutoWorker) finishedRunByPerson(ctx context.Context, job domain.PlanAutoJob) bool {
	recent, err := w.runs.ListRecent(ctx, job.RequestID, planAutoRecentRuns)
	if err != nil {
		return false
	}
	for _, run := range recent {
		if run.Kind != domain.RunKindPlan {
			continue
		}
		return !ownsRun(job, run) && run.Status != domain.RunStatusRunning && !run.StartedAt.Before(job.CreatedAt)
	}
	return false
}
