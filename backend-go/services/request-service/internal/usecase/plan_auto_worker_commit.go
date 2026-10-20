package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// commitPlan is the committing step: write the proposal of the finished run as the Request's Plan. An AI failure is
// never repeated from here; only the write is.
func (w *PlanAutoWorker) commitPlan(ctx context.Context, job domain.PlanAutoJob) {
	if job.RunID == "" {
		w.finish(ctx, job, domain.PlanAutoGaveUp, domain.PlanAutoCodeCommitFailed)
		return
	}
	run, err := w.runs.Get(ctx, job.RunID)
	if isNotFoundKind(err) {
		w.finish(ctx, job, domain.PlanAutoSuperseded, planAutoReasonNewerRun)
		return
	}
	if err != nil {
		w.retryLater(ctx, job, err)
		return
	}
	if run.Status != domain.RunStatusSucceeded {
		w.finish(ctx, job, domain.PlanAutoGaveUp, domain.PlanAutoCodeCommitFailed)
		return
	}
	proposal, err := DecodePlanProposal(derefStr(run.RawOutput))
	if err != nil {
		// A stored proposal that cannot be read will not become readable; running the AI again is a decision for a person.
		slog.WarnContext(ctx, "stored plan proposal unreadable", slog.String("request_id", job.RequestID), slog.String("run_id", run.ID))
		w.finish(ctx, job, domain.PlanAutoGaveUp, domain.PlanAutoCodeCommitFailed)
		return
	}
	_, err = w.commit.ExecuteForSystem(ctx, CommitPlanInput{RequestID: job.RequestID, Proposal: proposal})
	switch {
	case err == nil:
		w.finish(ctx, job, domain.PlanAutoDone, "")
	case errorHasCode(err, "REQUEST_STATE_STALE"):
		w.afterStaleCommit(ctx, job)
	case permanentKind(err):
		w.finish(ctx, job, domain.PlanAutoGaveUp, errorCode(err))
	default:
		w.commitFailed(ctx, job, err)
	}
}

// afterStaleCommit tells "the other side committed" from "the Request moved on" by reading the Request again.
func (w *PlanAutoWorker) afterStaleCommit(ctx context.Context, job domain.PlanAutoJob) {
	req, err := w.requests.Get(ctx, job.RequestID)
	if err != nil {
		w.retryLater(ctx, job, err)
		return
	}
	if req.Status == domain.RequestStatusAwaitingPlanApproval && req.PlanTaskID != "" {
		w.finish(ctx, job, domain.PlanAutoDone, "")
		return
	}
	w.finish(ctx, job, domain.PlanAutoSuperseded, planAutoReasonMoved)
}

// commitFailed counts the failure against the commit budget; the run is kept and the next attempt writes the same proposal.
func (w *PlanAutoWorker) commitFailed(ctx context.Context, job domain.PlanAutoJob, cause error) {
	job.CommitAttempts++
	if job.CommitAttempts >= w.policy.MaxCommitAttempts {
		slog.WarnContext(ctx, "plan commit gave up", slog.String("request_id", job.RequestID), slog.Any("error", cause))
		w.finish(ctx, job, domain.PlanAutoGaveUp, domain.PlanAutoCodeCommitFailed)
		return
	}
	slog.WarnContext(ctx, "plan commit failed; will be tried again", slog.String("request_id", job.RequestID), slog.Int("commit_attempts", job.CommitAttempts), slog.Any("error", cause))
	w.reschedule(ctx, job, domain.PlanAutoCommitting, w.policy.RetryDelay)
}
