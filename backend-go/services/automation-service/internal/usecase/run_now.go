package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// runTimeout is the outer deadline bounding an entire dispatched run's
// action loop — BR-AT-06. Distinct from workflow-service's own per-step
// 30-minute deadline: this bounds the WHOLE chain. Package-level var (not
// const) so RunNow.timeout can default to it while still being overridable
// per-instance for tests.
var runTimeout = 2 * time.Hour

// keepRuns is BR-AT-07's retention cap — the most recent N automation_runs
// rows are kept per automation, older ones pruned best-effort after every
// dispatch.
const keepRuns = 30

// RunNow is THE core interactor of this service — see
// specs/backend-go/services/automation-service.md §2/§6. It is the only
// code path (scheduler ticks, direct RunNow calls, and event-triggered
// dispatches all funnel through it) that dispatches a step, and it does so
// by calling out to workflow-service.ExecuteAdHocStep over real gRPC
// (WorkflowStepExecutor), never by executing anything locally. This closes
// TS Gap 3: TS's automation.runNow had no working dispatcher and every
// triggered run resolved skipped_unavailable.
type RunNow struct {
	automations AutomationRepository
	runs        AutomationRunRepository
	executor    WorkflowStepExecutor
	logger      *slog.Logger
	// timeout defaults to runTimeout — same-package tests shorten it
	// directly (uc.timeout = ...) rather than waiting 2 real hours.
	timeout time.Duration
}

func NewRunNow(automations AutomationRepository, runs AutomationRunRepository, executor WorkflowStepExecutor) *RunNow {
	return &RunNow{automations: automations, runs: runs, executor: executor, logger: slog.Default(), timeout: runTimeout}
}

// RunNowInput mirrors RunNowRequest (proto/orca/automation/v1/automation.proto),
// plus Trigger — not on the wire message itself, set by each caller: the
// gRPC RunNow handler passes RunTriggerManual, internal/adapter/scheduler
// passes RunTriggerScheduled, HandleEventTrigger passes RunTriggerEvent,
// HandleExternalTrigger passes RunTriggerExternal. Left empty, it defaults
// to RunTriggerManual (see Execute) so existing manual-only callers don't
// need to change.
type RunNowInput struct {
	AutomationID string
	RequestID    string // idempotency key — see automation-service.md §8
	Trigger      domain.RunTrigger
}

func (uc *RunNow) Execute(ctx context.Context, in RunNowInput) (domain.AutomationRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindUnauthenticated, "AUTOMATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.RequestID == "" {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_NO_REQUEST_ID", "request_id is required for idempotent dispatch", nil)
	}

	automation, err := uc.automations.Get(ctx, tenantID, in.AutomationID)
	if err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindNotFound, "AUTOMATION_NOT_FOUND", "automation not found", err)
	}

	// Idempotency check, per automation-service.md §8: a retried or
	// duplicate-ticked dispatch for the same (automation_id, request_id)
	// returns the existing run instead of calling workflow-service again.
	if existing, found, err := uc.runs.FindByRequestID(ctx, tenantID, automation.ID, in.RequestID); err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_LOOKUP_FAILED", "failed to check run idempotency", err)
	} else if found {
		return existing, nil
	}

	// step_type/step_config_json mirror Actions[0] (see
	// domain.NewAutomation) — kept for AutomationRun's own back-compat
	// top-level fields.
	stepType := automation.StepType
	if !stepType.Valid() {
		stepType = domain.StepTypeAgent
	}

	trigger := in.Trigger
	if !trigger.Valid() {
		trigger = domain.RunTriggerManual
	}

	now := time.Now().UTC()
	pending, err := domain.NewPendingRun(uuid.NewString(), automation.ID, tenantID, in.RequestID, stepType, trigger, automation.StepConfigJSON, now)
	if err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_RUN_INVALID", err.Error(), err)
	}

	if err := uc.runs.Create(ctx, pending); err != nil {
		// A unique-constraint race on (automation_id, request_id) is
		// expected occasionally under at-least-once scheduling (two
		// replicas ticking the same due automation) — re-check once before
		// treating it as a real failure, per §8's "idempotency key makes a
		// duplicate claim harmless" note.
		if existing, found, ferr := uc.runs.FindByRequestID(ctx, tenantID, automation.ID, in.RequestID); ferr == nil && found {
			return existing, nil
		}
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_CREATE_FAILED", "failed to persist automation run", err)
	}

	running, err := pending.MarkRunning(time.Now().UTC())
	if err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_TRANSITION_FAILED", "failed to transition run to running", err)
	}
	if err := uc.runs.UpdateStatus(ctx, running); err != nil {
		// BR-AT-08: a concurrent dispatch (manual RunNow racing the
		// scheduler ticker, or two replicas' ticks) already claimed the
		// 'running' slot for this automation — return the winner's run
		// instead of treating this as a real failure.
		if errors.Is(err, ErrConcurrentRunActive) {
			if existing, found, ferr := uc.runs.FindRunning(ctx, tenantID, automation.ID); ferr == nil && found {
				return existing, nil
			}
		}
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
	}

	timeout := uc.timeout
	if timeout <= 0 {
		timeout = runTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// THE cross-service call this whole service exists for: delegate step
	// execution to workflow-service, never execute it here. Dispatches
	// automation.Actions in order (BR-AT-01), honoring each action's
	// OnFailure policy.
	var results []domain.ActionResult
	runFailed := false
	timedOut := false
	// lastTransportErr is set only by a technical dispatch failure (the
	// executor call itself erroring — workflow-service unreachable, etc.),
	// never by a business-level step failure (result.Status == "failed").
	// RunNow fails closed (returns an error to its caller) only for the
	// former, per automation-service.md §8 — a business-level step failure
	// is recorded on the run but is not itself a RunNow error.
	var lastTransportErr error
	for i, action := range automation.Actions {
		result, execErr := uc.executor.ExecuteAdHocStep(runCtx, ExecuteAdHocStepInput{
			TenantID:       tenantID,
			StepType:       action.StepType,
			StepConfigJSON: action.StepConfigJSON,
			RequestID:      fmt.Sprintf("%s:%d", in.RequestID, i), // per-action idempotency suffix
		})

		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			timedOut = true
			runFailed = true
			ar := domain.ActionResult{Index: i, Status: "failed", ErrorMessage: "automation run exceeded 2h timeout"}
			results = append(results, ar)
			break
		}

		ar := domain.ActionResult{Index: i}
		switch {
		case execErr != nil:
			ar.Status = "failed"
			ar.ErrorMessage = execErr.Error()
			lastTransportErr = execErr
		case result.Status == "failed":
			ar.Status = "failed"
			ar.ErrorMessage = result.OutputJSON
			ar.OutputJSON = result.OutputJSON
			lastTransportErr = nil
		default:
			ar.Status = "succeeded"
			ar.OutputJSON = result.OutputJSON
			lastTransportErr = nil
		}
		results = append(results, ar)

		if ar.Status == "failed" {
			policy := action.OnFailure
			if policy == "" {
				policy = domain.OnFailureStop
			}
			if policy == domain.OnFailureStop {
				runFailed = true
				break
			}
		}
	}

	completedAt := time.Now().UTC()
	last := domain.ActionResult{}
	if len(results) > 0 {
		last = results[len(results)-1]
	}

	var final domain.AutomationRun
	if runFailed {
		reason := last.ErrorMessage
		if timedOut {
			reason = "automation run exceeded 2h timeout"
		}
		failed, ferr := running.MarkFailed(completedAt, reason)
		if ferr != nil {
			return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_TRANSITION_FAILED", "failed to transition run to failed", ferr)
		}
		failed.OutputJSON = last.OutputJSON
		failed.ActionResults = results
		final = failed
	} else {
		succeeded, serr := running.MarkSucceeded(completedAt, last.OutputJSON)
		if serr != nil {
			return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_TRANSITION_FAILED", "failed to transition run to succeeded", serr)
		}
		succeeded.ActionResults = results
		final = succeeded
	}

	// ctx, not runCtx — runCtx may already be expired on a timeout, and
	// persisting the terminal status must still succeed.
	if err := uc.runs.UpdateStatus(ctx, final); err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
	}

	// BR-AT-07 — best-effort retention prune, never fails the run itself.
	if err := uc.runs.PruneOldRuns(ctx, tenantID, automation.ID, keepRuns); err != nil {
		uc.logger.Warn("failed to prune old automation runs", "error", err, "automation_id", automation.ID)
	}

	switch {
	case timedOut:
		return final, apperrors.New(apperrors.KindDeadlineExceeded, "AUTOMATION_RUN_TIMEOUT", "run exceeded 2h timeout", runCtx.Err())
	case runFailed && lastTransportErr != nil:
		// Fail closed, per automation-service.md §8 — availability of
		// CRUD/list stays independent of workflow-service, but a run that
		// couldn't be dispatched is recorded Failed, never silently
		// swallowed the way TS's skipped_unavailable was.
		return final, apperrors.New(apperrors.KindInternal, "AUTOMATION_WORKFLOW_UNAVAILABLE", "workflow-service call failed", lastTransportErr)
	default:
		// Either succeeded, or failed at the business level (the executor
		// itself was reachable and reported result.Status == "failed") —
		// not a RunNow error.
		return final, nil
	}
}
