package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ExecuteInput mirrors ExecuteRequest. ProjectID is persisted on the
// resulting execution — see domain.WorkflowExecution and
// usecase.HasActiveExecutions, which is what project-service.RebindDevServer
// relies on (Epic C, backend-go/docs/execution-plan.md).
type ExecuteInput struct {
	TemplateID  string
	ProjectID   string
	RootTraceID string
	RequestID   string
}

// Execute resolves a template, validates and wave-computes its DAG,
// records a new execution (status=running), and dispatches wave 0 onward —
// workflow-service.md §7's full dependency diagram, closing the gap the
// previous scaffold left ("a recorded execution never progresses past
// running on its own").
//
// Architectural decision: Execute dispatches ASYNCHRONOUSLY. It validates
// the DAG and builds waves synchronously (a cyclic or otherwise invalid DAG
// must fail the RPC, not fail silently in the background), persists the
// execution row, then hands the built waves to waveDispatcher on a
// detached background goroutine and returns immediately — it does NOT
// block the RPC until the whole DAG finishes. Reasons:
//   - A DAG can contain long-running steps (agent/shell relayed through
//     infra-fleet-service, §8: default 30-minute step timeout) — blocking
//     Execute's RPC on that would blow past any sane gRPC deadline.
//   - This directly targets the bug this pass fixes: "an execution never
//     progresses past running on its own" — the fix is that dispatch runs
//     independently of the RPC that started it, not that the RPC waits.
//   - It fits the resumability model §8 describes: a running execution's
//     progress is driven by dispatch (today: the goroutine started here;
//     later: a boot-time recovery scan re-attaching to root_trace_id, not
//     implemented in this pass, see README), never by an RPC caller
//     holding a connection open.
//
// The background goroutine is handed a context carrying the resolved
// tenant ID but detached from the inbound RPC's context — ctx is cancelled
// the moment this handler returns, which must not cancel a dispatch that's
// only just gotten started.
type Execute struct {
	templates      TemplateRepository
	executions     ExecutionRepository
	stepExecutions StepExecutionRepository
	dispatcher     *waveDispatcher
}

func NewExecute(templates TemplateRepository, executions ExecutionRepository, stepExecutions StepExecutionRepository, registry StepExecutorRegistry) *Execute {
	return &Execute{
		templates:      templates,
		executions:     executions,
		stepExecutions: stepExecutions,
		dispatcher:     newWaveDispatcher(stepExecutions, registry, defaultMaxConcurrentSteps),
	}
}

func (uc *Execute) Execute(ctx context.Context, in ExecuteInput) (domain.WorkflowExecution, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	tmpl, err := uc.templates.GetTemplate(ctx, tenantID, in.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return domain.WorkflowExecution{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found", err)
		}
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_TEMPLATE_FETCH_FAILED", "failed to fetch workflow template", err)
	}

	dag, err := domain.ParseDAG(tmpl.DAGJSON)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_DAG", err.Error(), err)
	}
	if err := dag.Validate(); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_DAG_VALIDATION_FAILED", err.Error(), err)
	}
	waves, err := dag.BuildWaves()
	if err != nil {
		// A cyclic DAG (BuildWaves' only failure mode once Validate has
		// already passed) must fail the RPC synchronously, same as any
		// other validation error — not be discovered only once dispatch
		// starts in the background below.
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_DAG_CYCLIC", err.Error(), err)
	}

	rootTraceID := in.RootTraceID
	if rootTraceID == "" {
		// No caller-supplied trace to resume against — mint a fresh one so
		// this execution is still resumable after a restart (§8's hard
		// requirement applies to every execution, not just ones the caller
		// happened to pass a root_trace_id for).
		rootTraceID = uuid.NewString()
	}

	exec, err := domain.NewWorkflowExecution(uuid.NewString(), tenantID, tmpl.ID, rootTraceID, in.ProjectID)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_EXECUTION", err.Error(), err)
	}

	if err := uc.executions.CreateExecution(ctx, exec); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_SAVE_FAILED", "failed to persist workflow execution", err)
	}

	// Detach from ctx (cancelled the instant this RPC handler returns) but
	// carry the resolved tenant id forward explicitly, since
	// StepExecutionRepository/ExecutionRepository calls made from the
	// background goroutine still need it — see this type's doc comment.
	dispatchCtx := tenant.WithTenantID(context.Background(), tenantID)
	go uc.runToCompletion(dispatchCtx, exec, waves)

	return exec, nil
}

// runToCompletion dispatches every wave of exec's DAG and persists exec's
// final status (completed if every wave succeeded, failed if any step
// did not — see waveDispatcher's doc comment for the failure-semantics
// rationale). Runs entirely off the originating RPC's goroutine.
// runToCompletion is the one place exec.Status transitions to a terminal
// value for the main dispatch path — the single outbox publish point
// SOL-PW-04 (TASK-PW-04-06) adds. A marshal failure degrades to "persist
// status, skip the event" rather than failing the whole terminal
// transition — matches this function's existing best-effort logging
// posture for UpdateExecution failures (both are already fire-and-forget
// from a background goroutine with no caller to propagate an error to).
func (uc *Execute) runToCompletion(ctx context.Context, exec domain.WorkflowExecution, waves [][]domain.Step) {
	succeeded := uc.dispatcher.dispatchWaves(ctx, exec.ID, waves)

	exec.Status = domain.StatusCompleted
	subject := "orca.workflow.execution.completed"
	if !succeeded {
		exec.Status = domain.StatusFailed
		subject = "orca.workflow.execution.failed"
	}

	payload, err := json.Marshal(workflowExecutionTerminalPayload{
		ExecutionID: exec.ID, TemplateID: exec.TemplateID, ProjectID: exec.ProjectID, Status: string(exec.Status),
	})
	var event *domain.OutboxEvent
	if err != nil {
		slog.ErrorContext(ctx, "workflow: marshaling terminal-status event payload failed", slog.String("execution_id", exec.ID), slog.Any("error", err))
	} else {
		event = &domain.OutboxEvent{ID: uuid.NewString(), Subject: subject, OccurredAt: time.Now().UTC(), PayloadJSON: payload}
	}

	if err := uc.executions.UpdateExecution(ctx, exec, event); err != nil {
		slog.ErrorContext(ctx, "workflow: persisting final execution status failed", slog.String("execution_id", exec.ID), slog.String("status", string(exec.Status)), slog.Any("error", err))
	}
}

type workflowExecutionTerminalPayload struct {
	ExecutionID string `json:"execution_id"`
	TemplateID  string `json:"template_id"`
	ProjectID   string `json:"project_id"`
	Status      string `json:"status"`
}
