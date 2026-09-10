package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// TickDispatch is the ONE usecase the tick loop (cmd/server/main.go) calls
// per interval — the internal engine orchestration-service.md §2.2
// describes as "the coordinator then autonomously ticking the run's state
// machine." No gRPC-facing counterpart exists for this usecase.
type TickDispatch struct {
	tasks    OrchestrationTaskRepository
	runs     CoordinatorRunRepository
	dispatch *CreateDispatchContext // reused as-is, not reimplemented
	worker   WorkerDispatcher
	fail     *FailDispatch // reused as-is for a dispatch call that errors
	reporter TaskServiceReporter
}

func NewTickDispatch(tasks OrchestrationTaskRepository, runs CoordinatorRunRepository, dispatch *CreateDispatchContext, worker WorkerDispatcher, fail *FailDispatch, reporter TaskServiceReporter) *TickDispatch {
	return &TickDispatch{tasks: tasks, runs: runs, dispatch: dispatch, worker: worker, fail: fail, reporter: reporter}
}

func (uc *TickDispatch) Execute(ctx context.Context) error {
	if err := uc.dispatchReady(ctx); err != nil {
		return err
	}
	uc.retryUnreported(ctx) // best-effort; never fails the whole tick
	return nil
}

// dispatchReady claims and dispatches every ready-unclaimed task this
// instance can see. A ready task with an unresolved decision gate never
// reaches this scan (gate resolution/CreateGate already transitions the
// owning task to blocked, per §4/§8 — see decision_gates' own atomicity
// contract), so this loop does not need its own gate check.
func (uc *TickDispatch) dispatchReady(ctx context.Context) error {
	ready, err := uc.tasks.ListReadyUnclaimed(ctx)
	if err != nil {
		return fmt.Errorf("tick: list ready tasks: %w", err)
	}
	for _, task := range ready {
		claimed, ok, err := uc.tasks.ClaimReady(ctx, task.TenantID, task.ID)
		if err != nil {
			slog.Error("tick: claim failed", slog.String("task_id", task.ID), slog.Any("error", err))
			continue // one task's claim failure must not abort the whole batch
		}
		if !ok {
			continue // lost the race to another tick/instance — not an error
		}
		// one worker per orchestration_task, per orchestration-service.md
		// §4's DispatchContext.assignee_handle shape
		handle := "worker:" + claimed.ID
		// The tick loop runs as the service's own internal identity (no
		// per-request caller — see ports.go's ListRunning/
		// ListUnreportedTerminal doc comments), but CreateDispatchContext's
		// usecase still requires tenant.RequireTenantID(ctx) like every
		// other RPC-facing usecase. Inject the claimed task's own tenant
		// here rather than widening CreateDispatchContext's contract for
		// this one internal caller.
		taskCtx := tenant.WithTenantID(ctx, claimed.TenantID)
		dc, err := uc.dispatch.Execute(taskCtx, CreateDispatchContextInput{
			Handle: handle, CoordinatorRunID: claimed.CoordinatorRunID, OrchestrationTaskID: claimed.ID,
		})
		if err != nil {
			slog.Error("tick: create dispatch context failed", slog.String("task_id", claimed.ID), slog.Any("error", err))
			continue // task stays 'dispatched' with no dispatch_context row; a reconciliation pass is a flagged follow-up, not solved here (see solution's "Not in scope")
		}
		if err := uc.worker.Dispatch(ctx, claimed.TenantID, claimed, handle); err != nil {
			// Real, permanent-until-retried dispatch failure — record it
			// via the EXISTING FailDispatch usecase (circuit-breaker logic
			// reused unmodified).
			_, ferr := uc.fail.Execute(taskCtx, FailDispatchInput{
				DispatchContextID: dc.ID, ErrorMessage: err.Error(), GRPCStatusCode: uint32(status.Code(err)),
			})
			if ferr != nil {
				slog.Error("tick: record dispatch failure also failed", slog.String("dispatch_context_id", dc.ID), slog.Any("error", ferr))
			}
			continue
		}
	}
	return nil
}

// retryUnreported re-attempts TaskServiceReporter.ReportResult for any run
// whose terminal state was persisted (UpdateTaskStatusAndPromote's tail,
// TASK-TASKV1-005-07) but never successfully reported — closes the retry
// gap that tail's own doc comment flags. Best-effort: a retry failure here
// is logged and left for the NEXT tick, same as the first attempt's own
// failure handling.
func (uc *TickDispatch) retryUnreported(ctx context.Context) {
	unreported, err := uc.runs.ListUnreportedTerminal(ctx)
	if err != nil {
		slog.Error("tick: list unreported terminal runs failed", slog.Any("error", err))
		return
	}
	for _, run := range unreported {
		success := run.Status == domain.RunStatusCompleted
		if err := uc.reporter.ReportResult(ctx, run.OriginTaskID, run.ID, success, run.ErrorMessage); err != nil {
			slog.Error("tick: retry report result failed", slog.String("coordinator_run_id", run.ID), slog.Any("error", err))
			continue
		}
		if err := uc.runs.MarkReported(ctx, run.TenantID, run.ID); err != nil {
			slog.Error("tick: mark reported failed", slog.String("coordinator_run_id", run.ID), slog.Any("error", err))
		}
	}
}
