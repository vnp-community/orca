package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// RunNowInput mirrors RunNowRequest (proto/orca/automation/v1/automation.proto),
// plus Trigger — not on the wire message itself, set by each caller: the
// gRPC RunNow handler passes RunTriggerManual, internal/adapter/scheduler
// passes RunTriggerScheduled, HandleExternalTrigger passes
// RunTriggerExternal. Left empty, it defaults to RunTriggerManual (see
// Execute) so existing manual-only callers don't need to change.
type RunNowInput struct {
	AutomationID string
	RequestID    string // idempotency key — see automation-service.md §8
	Trigger      domain.RunTrigger
}

// RunNow is THE core interactor of this service — see
// specs/backend-go/services/automation-service.md §2/§6. It is the only
// code path (scheduler ticks and direct RunNow calls both funnel through
// it) that dispatches a step, and it does so by calling out to
// workflow-service.ExecuteAdHocStep over real gRPC (WorkflowStepExecutor),
// never by executing anything locally. This closes TS Gap 3: TS's
// automation.runNow had no working dispatcher and every triggered run
// resolved skipped_unavailable.
type RunNow struct {
	automations AutomationRepository
	runs        AutomationRunRepository
	executor    WorkflowStepExecutor
}

func NewRunNow(automations AutomationRepository, runs AutomationRunRepository, executor WorkflowStepExecutor) *RunNow {
	return &RunNow{automations: automations, runs: runs, executor: executor}
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

	// step_type is now a first-class stored column on Automation (migration
	// 0002) rather than a key inside step_config_json — see
	// domain.NewAutomation's doc comment. It's already guaranteed valid
	// (NewAutomation defaults StepTypeUnspecified to StepTypeAgent at
	// creation time), but default again defensively here in case a row
	// predates that migration and still has an empty step_type.
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
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
	}

	// THE cross-service call this whole service exists for: delegate step
	// execution to workflow-service, never execute it here.
	result, execErr := uc.executor.ExecuteAdHocStep(ctx, ExecuteAdHocStepInput{
		TenantID:       tenantID,
		StepType:       stepType,
		StepConfigJSON: automation.StepConfigJSON,
		RequestID:      in.RequestID,
	})
	if execErr != nil {
		// Fail closed, per automation-service.md §8 — availability of
		// CRUD/list stays independent of workflow-service, but a run that
		// couldn't be dispatched is recorded Failed, never silently
		// swallowed the way TS's skipped_unavailable was.
		if failed, ferr := running.MarkFailed(time.Now().UTC(), execErr.Error()); ferr == nil {
			_ = uc.runs.UpdateStatus(ctx, failed) // best-effort; the transport error below is still returned either way
		}
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_WORKFLOW_UNAVAILABLE", "workflow-service call failed", execErr)
	}

	if result.Status == "failed" {
		failed, err := running.MarkFailed(time.Now().UTC(), result.OutputJSON)
		if err != nil {
			return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_TRANSITION_FAILED", "failed to transition run to failed", err)
		}
		if err := uc.runs.UpdateStatus(ctx, failed); err != nil {
			return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
		}
		return failed, nil
	}

	succeeded, err := running.MarkSucceeded(time.Now().UTC(), result.OutputJSON)
	if err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_TRANSITION_FAILED", "failed to transition run to succeeded", err)
	}
	if err := uc.runs.UpdateStatus(ctx, succeeded); err != nil {
		return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
	}
	return succeeded, nil
}
