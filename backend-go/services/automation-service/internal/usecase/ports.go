// Package usecase holds automation-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// AutomationRepository is the persistence port for automation definitions.
// Implemented by internal/adapter/postgres against automation-service's own
// database — see architecture/05-data-architecture.md's
// database-per-service rule.
type AutomationRepository interface {
	Create(ctx context.Context, automation domain.Automation) error
	Get(ctx context.Context, tenantID, id string) (domain.Automation, error)
}

// AutomationRunRepository is the persistence port for run bookkeeping.
type AutomationRunRepository interface {
	// Create inserts a new run row. Callers must check FindByRequestID first
	// (see RunNow) — Create is not itself idempotent, though the unique
	// index on (automation_id, request_id) backstops a race per
	// automation-service.md §8.
	Create(ctx context.Context, run domain.AutomationRun) error
	// FindByRequestID looks up an existing run for (tenantID, automationID,
	// requestID) — the idempotency check RunNow performs before dispatching,
	// so a retried or duplicate-ticked call returns the existing run instead
	// of triggering a second workflow-service execution.
	FindByRequestID(ctx context.Context, tenantID, automationID, requestID string) (domain.AutomationRun, bool, error)
	// UpdateStatus persists a run's status transition (and whichever of
	// StartedAt/CompletedAt/OutputJSON/ErrorMessage changed).
	UpdateStatus(ctx context.Context, run domain.AutomationRun) error
	ListByAutomation(ctx context.Context, tenantID, automationID, pageToken string, pageSize int32) ([]domain.AutomationRun, string, error)
}

// ExecuteAdHocStepInput mirrors workflow-service's ExecuteAdHocStepRequest
// (see proto/orca/workflow/v1/workflow.proto) at the usecase boundary, so
// this package never imports workflowv1 directly — only
// internal/adapter/grpcclient does that translation.
type ExecuteAdHocStepInput struct {
	TenantID       string
	StepType       domain.StepType
	StepConfigJSON string
	RequestID      string
}

// ExecuteAdHocStepOutput mirrors workflow-service's StepResult.
type ExecuteAdHocStepOutput struct {
	Status     string // "completed"|"failed", as reported by workflow-service
	OutputJSON string
}

// DueAutomationClaimer is the persistence port internal/adapter/scheduler's
// ticker loop uses to atomically claim due automations — kept distinct from
// AutomationRepository since its SKIP LOCKED claim semantics and the
// "hold the claim until dispatch is confirmed, then advance" lifecycle
// (see ClaimedBatch) are scheduler-specific, not something
// CreateAutomation/RunNow need.
type DueAutomationClaimer interface {
	// ClaimDue locks up to limit enabled automations whose next_run_at is
	// at or before now, via SELECT ... FOR UPDATE SKIP LOCKED, so
	// concurrent replicas' ticks never claim the same due row — see
	// automation-service.md §7. The returned batch's transaction is left
	// open; the caller must Commit or Rollback it (see ClaimedBatch).
	ClaimDue(ctx context.Context, now time.Time, limit int32) (ClaimedBatch, error)
}

// ClaimedBatch is a still-open claim transaction returned by ClaimDue.
// Deliberately kept open across dispatch: automation-service.md §8 requires
// at-least-once semantics ("a missed tick must not silently drop a run"),
// so a due row must stay claimable-again (i.e. its next_run_at must not
// advance) until dispatch is confirmed — if the process crashes between
// ClaimDue and Advance/Commit, the transaction rolls back, the lock is
// released, and the next tick picks the same occurrence back up rather than
// silently losing it.
type ClaimedBatch interface {
	// Automations returns the locked, due rows claimed by ClaimDue.
	Automations() []domain.Automation
	// Advance persists automationID's next next_run_at within the SAME
	// transaction that holds the claim lock. hasNext=false (an
	// exhausted COUNT/UNTIL-bounded rule) persists NULL, which naturally
	// excludes the row from future due-queries without needing to also
	// flip Enabled.
	Advance(ctx context.Context, automationID string, nextRunAt time.Time, hasNext bool) error
	// Commit persists all Advance calls and releases the claim locks.
	Commit(ctx context.Context) error
	// Rollback discards the batch — locks release, next_run_at values are
	// unchanged, so every claimed row is due again on the next tick.
	Rollback(ctx context.Context) error
}

// WorkflowStepExecutor is THE port this service exists to call for real —
// see specs/backend-go/services/automation-service.md §2/§6: RunNow
// delegates step execution to workflow-service.ExecuteAdHocStep over gRPC
// (internal/adapter/grpcclient/workflow_client.go), never reimplementing
// step dispatch locally. A future contributor adding a "convenience" local
// executor instead of going through this port would silently reintroduce
// the exact gap (TS Gap 3) this redesign closes.
type WorkflowStepExecutor interface {
	ExecuteAdHocStep(ctx context.Context, in ExecuteAdHocStepInput) (ExecuteAdHocStepOutput, error)
}
