package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Reasons recorded on a job that a person got ahead of; they are not run error codes.
const (
	planAutoReasonMoved     = "REQUEST_PLAN_REQUEST_MOVED"
	planAutoReasonPlanExist = "REQUEST_PLAN_EXISTS"
	planAutoReasonNewerRun  = "REQUEST_PLAN_NEWER_RUN"
	planAutoRecentRuns      = 10
	planAutoCodeBusy        = "REQUEST_ANALYSIS_BUSY"
)

// PlanAutoStarter starts the Plan run on the system's behalf (GeneratePlan.ExecuteForSystem).
type PlanAutoStarter interface {
	ExecuteForSystem(ctx context.Context, in GeneratePlanInput) (GeneratePlanResult, error)
}

// PlanAutoCommitter commits a proposal the server holds (CommitPlan.ExecuteForSystem).
type PlanAutoCommitter interface {
	ExecuteForSystem(ctx context.Context, in CommitPlanInput) (CommitPlanResult, error)
}

var (
	_ PlanAutoStarter   = (*GeneratePlan)(nil)
	_ PlanAutoCommitter = (*CommitPlan)(nil)
)

// PlanAutoWorker drives the stored jobs of automatic Plan generation (CR-REQ-044): pending jobs start a run, running
// jobs watch it, committing jobs write the Plan. The database is the schedule, because the event bus redelivers at
// once and cannot wait. Every step reloads the Request, so a person who got there first is never fought.
type PlanAutoWorker struct {
	jobs     PlanAutoJobRepository
	requests RequestRepository
	runs     AnalysisRunStore
	start    PlanAutoStarter
	commit   PlanAutoCommitter
	sw       PlanAutoSwitch
	policy   domain.PlanAutoPolicy
	owner    string
	now      func() time.Time
}

type PlanAutoWorkerDeps struct {
	Jobs     PlanAutoJobRepository
	Requests RequestRepository
	Runs     AnalysisRunStore
	Start    PlanAutoStarter
	Commit   PlanAutoCommitter
	// Switch is asked again before a run starts, so turning the project switch off stops jobs that are still waiting.
	Switch PlanAutoSwitch
	Policy domain.PlanAutoPolicy
	Owner  string
}

func NewPlanAutoWorker(d PlanAutoWorkerDeps) *PlanAutoWorker {
	return &PlanAutoWorker{jobs: d.Jobs, requests: d.Requests, runs: d.Runs, start: d.Start, commit: d.Commit, sw: d.Switch, policy: d.Policy, owner: d.Owner,
		now: func() time.Time { return time.Now().UTC() }}
}

// RunLoop polls until ctx ends. A failed pass is logged and tried again on the next tick.
func (w *PlanAutoWorker) RunLoop(ctx context.Context, interval time.Duration, batch int) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := w.RunOnce(ctx, batch); err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "plan auto-generation pass failed", slog.Any("error", err))
			}
		}
	}
}

// RunOnce claims the jobs that are due and handles each in its own goroutine; it returns how many it claimed.
func (w *PlanAutoWorker) RunOnce(ctx context.Context, batch int) (int, error) {
	jobs, err := w.jobs.ClaimDue(ctx, w.owner, w.policy.LeaseTTL, batch)
	if err != nil {
		return 0, err
	}
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					slog.ErrorContext(ctx, "plan auto-generation step panicked", slog.String("request_id", job.RequestID), slog.Any("panic", p))
				}
			}()
			jobCtx, cancel := context.WithTimeout(tenant.WithTenantID(ctx, job.TenantID), w.policy.LeaseTTL)
			defer cancel()
			w.process(jobCtx, job)
		}()
	}
	wg.Wait()
	return len(jobs), nil
}

func (w *PlanAutoWorker) process(ctx context.Context, job domain.PlanAutoJob) {
	req, err := w.requests.Get(ctx, job.RequestID)
	if isNotFound(err) {
		w.finish(ctx, job, domain.PlanAutoSuperseded, planAutoReasonMoved)
		return
	}
	if err != nil {
		w.retryLater(ctx, job, err)
		return
	}
	if superseded, reason := w.supersededBy(req, job); superseded {
		if job.State == domain.PlanAutoCommitting && req.Status == domain.RequestStatusAwaitingPlanApproval && req.PlanTaskID != "" {
			w.finish(ctx, job, domain.PlanAutoDone, "") // the Plan the job was writing is already there
			return
		}
		w.finish(ctx, job, domain.PlanAutoSuperseded, reason)
		return
	}
	switch job.State {
	case domain.PlanAutoPending:
		w.startRun(ctx, job, req)
	case domain.PlanAutoRunning:
		w.observeRun(ctx, job)
	case domain.PlanAutoCommitting:
		w.commitPlan(ctx, job)
	default:
		w.release(ctx, job)
	}
}

// supersededBy applies the rules that make the job step aside. The "already has a Plan" rule is for the jobs that
// create the first Plan; a job that revises an existing Plan (CR-REQ-045) legitimately finds one.
func (w *PlanAutoWorker) supersededBy(req domain.Request, job domain.PlanAutoJob) (bool, string) {
	if req.Status != domain.RequestStatusPlanning {
		return true, planAutoReasonMoved
	}
	if req.PlanTaskID != "" && (job.Origin == domain.PlanAutoOriginEvent || job.Origin == domain.PlanAutoOriginRetry) {
		return true, planAutoReasonPlanExist
	}
	return false, ""
}

// finish records a final state, with the structured line that stands in for a metric.
func (w *PlanAutoWorker) finish(ctx context.Context, job domain.PlanAutoJob, state domain.PlanAutoState, reason string) {
	attempt := job.Attempt
	if !w.move(ctx, job, state, reason, 0) {
		return
	}
	slog.InfoContext(ctx, "plan auto-generation job finished",
		slog.String("request_id", job.RequestID), slog.String("origin", string(job.Origin)), slog.String("state", string(state)),
		slog.String("reason_code", reason), slog.Int("attempt", attempt))
}

// reschedule moves the job to next (often the state it is in) and asks to be looked at again after delay.
func (w *PlanAutoWorker) reschedule(ctx context.Context, job domain.PlanAutoJob, next domain.PlanAutoState, delay time.Duration) bool {
	return w.move(ctx, job, next, job.ReasonCode, delay)
}

// release gives the lease back unchanged; used when there is nothing to do for the state.
func (w *PlanAutoWorker) release(ctx context.Context, job domain.PlanAutoJob) {
	w.save(ctx, job)
}

// move writes the job in state to if the state machine allows it from the state the step began in. It returns false when
// nothing was stored, either because the lease is gone or the store failed; the step then stops and the lease expiry
// hands the job to a later pass.
func (w *PlanAutoWorker) move(ctx context.Context, job domain.PlanAutoJob, to domain.PlanAutoState, reason string, delay time.Duration) bool {
	if !job.State.CanMove(to) {
		slog.ErrorContext(ctx, "plan auto-generation job cannot make that move", slog.String("request_id", job.RequestID), slog.String("from", string(job.State)), slog.String("to", string(to)))
		return false
	}
	job.State, job.ReasonCode, job.NextCheckAt = to, reason, w.now().Add(delay)
	return w.save(ctx, job)
}

func (w *PlanAutoWorker) save(ctx context.Context, job domain.PlanAutoJob) bool {
	ok, err := w.jobs.Update(ctx, job, w.owner)
	if err != nil {
		slog.WarnContext(ctx, "plan auto-generation job not saved", slog.String("request_id", job.RequestID), slog.String("state", string(job.State)), slog.Any("error", err))
		return false
	}
	if !ok {
		slog.DebugContext(ctx, "plan auto-generation job lease lost", slog.String("request_id", job.RequestID))
	}
	return ok
}

// retryLater handles a failure of the infrastructure around the step (database, network): wait and try again, but
// not forever for a job that has not got a run yet.
func (w *PlanAutoWorker) retryLater(ctx context.Context, job domain.PlanAutoJob, cause error) {
	code := errorCode(cause)
	if code == "" {
		code = domain.RunErrInternal
	}
	if job.State == domain.PlanAutoPending && w.now().Sub(job.CreatedAt) > w.policy.StartWindow {
		w.finish(ctx, job, domain.PlanAutoGaveUp, code)
		return
	}
	slog.WarnContext(ctx, "plan auto-generation step failed; will be tried again", slog.String("request_id", job.RequestID), slog.String("state", string(job.State)), slog.Any("error", cause))
	w.reschedule(ctx, job, job.State, w.policy.RetryDelay)
}

// permanentKind reports errors that repeating the same call cannot fix.
func permanentKind(err error) bool {
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Kind {
	case apperrors.KindFailedPrecondition, apperrors.KindInvalidArgument, apperrors.KindNotFound, apperrors.KindPermissionDenied:
		return true
	}
	return false
}

// ownsRun reports whether the run was started by this job (its keys are plan-auto-<anchor>[-n]).
func ownsRun(job domain.PlanAutoJob, run domain.AnalysisRun) bool {
	if run.ID == job.RunID {
		return true
	}
	return run.IdempotencyKey != nil && strings.HasPrefix(*run.IdempotencyKey, "plan-auto-"+job.Anchor)
}
