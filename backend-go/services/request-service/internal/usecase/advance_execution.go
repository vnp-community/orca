package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// executionTaskTypes are the task types one ListTasks call must name: plan and phase are hidden unless asked for.
var executionTaskTypes = []string{"task", "bug", "feature", "epic", domain.TaskTypePlan, domain.TaskTypePhase}

type AdvanceInput struct {
	RequestID string
	// ContainerID limits the round to one Phase (or the Plan); empty means everything in scope.
	ContainerID string
	// ActorID is the approving user task-service checks the "execute" permission of.
	ActorID string
}

type AdvanceResult struct {
	Dispatched []string
	// GatesOpened lists the tasks held back by a type-policy gate, whose Approval is now pending.
	GatesOpened []string
}

// AdvanceExecution dispatches the ready working tasks of a Request. It is idempotent: running it again
// dispatches nothing for tasks that are already running, so StartPhase, the status consumer, the outcome
// consumer and the reconcile loop may all call it.
type AdvanceExecution struct {
	Requests    RequestReader
	Tasks       TaskClient
	Outcomes    TaskRunOutcomeRepository
	PhaseStarts PhaseStartRepository
	Policies    *domain.PolicyRegistry
	// Approvals opens the Approval a type-policy gate asks for; nil refuses to dispatch gated tasks.
	Approvals ApprovalOpener
	Settings  ExecutionSettings
	Log       *slog.Logger
	Now       func() time.Time

	locks keyedLock
}

func (uc *AdvanceExecution) now() time.Time {
	if uc.Now != nil {
		return uc.Now().UTC()
	}
	return time.Now().UTC()
}

func (uc *AdvanceExecution) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

// lockRequest serialises the loop for one Request in this process.
func (uc *AdvanceExecution) lockRequest(tenantID, requestID string) func() {
	return uc.locks.Lock(tenantID + "/" + requestID)
}

func (uc *AdvanceExecution) Execute(ctx context.Context, in AdvanceInput) (AdvanceResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return AdvanceResult{}, domain.ErrRequestTenantRequired()
	}
	defer uc.lockRequest(tenantID, in.RequestID)()
	req, err := uc.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return AdvanceResult{}, err
	}
	if req.Status != domain.RequestStatusExecuting {
		return AdvanceResult{}, domain.ErrRequestNotExecuting(req.Status)
	}
	tasks, err := uc.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return AdvanceResult{}, err
	}
	return uc.run(ctx, req, domain.BuildExecutionTree(tasks), in)
}

// run is Execute without the lock and the reads; EvaluateExecution calls it holding both.
func (uc *AdvanceExecution) run(ctx context.Context, req domain.Request, tree domain.ExecutionTree, in AdvanceInput) (AdvanceResult, error) {
	settings := uc.Settings.normalized()
	started, err := startedPhases(ctx, uc.PhaseStarts, req.ID)
	if err != nil {
		return AdvanceResult{}, err
	}
	var scope []domain.TaskView
	inProgress := 0
	for _, l := range tree.Leaves {
		if l.Status == domain.TaskStatusInProgress {
			inProgress++
		}
		if in.ContainerID != "" && l.ParentID != in.ContainerID {
			continue
		}
		if tree.InScope(l, started) {
			scope = append(scope, l)
		}
	}
	slots := settings.MaxParallelTasks - inProgress
	if slots <= 0 {
		return AdvanceResult{}, nil
	}
	policy := uc.Policies.PolicyFor(req.Type)
	shared := sharedWorktree(tree)
	execCtx := tenant.WithUserID(ctx, in.ActorID)

	var res AdvanceResult
	var errs []error
	for _, task := range scope {
		if task.Status != domain.TaskStatusOpen {
			continue
		}
		if slots <= 0 {
			break
		}
		failed, err := uc.Outcomes.CountFailed(ctx, task.ID)
		if err != nil {
			return res, err
		}
		if failed >= settings.MaxTaskAttempts {
			continue // out of attempts: EvaluateExecution sends the Request back, dispatching again would hide that
		}
		gate, err := policy.PreExecutionGate(ctx, req, task.Ref())
		if err != nil {
			return res, err
		}
		if gate != nil {
			if err := uc.openGate(ctx, req, *gate, in.ActorID); err != nil {
				errs = append(errs, fmt.Errorf("gate for task %s: %w", task.ID, err))
				continue
			}
			res.GatesOpened = append(res.GatesOpened, task.ID)
			continue
		}
		if shared != "" && task.WorktreeID != shared {
			// One worktree per Plan so a later task sees the code of the earlier ones (CR-REQ-013 2.7).
			if err := uc.Tasks.SetWorktree(execCtx, task.ID, shared); err != nil {
				return res, fmt.Errorf("share worktree with task %s: %w", task.ID, err)
			}
		}
		execRef := fmt.Sprintf("req:%s:%s:%d", req.ID, task.ID, failed+1)
		switch err := uc.Tasks.Execute(execCtx, task.ID, execRef); {
		case err == nil, errors.Is(err, domain.ErrTaskAlreadyRunning):
			res.Dispatched = append(res.Dispatched, task.ID)
			slots--
		case errors.Is(err, domain.ErrTaskForbidden):
			return res, domain.ErrExecuteForbidden()
		case errors.Is(err, domain.ErrTaskDispatchTransient):
			// Not an attempt: the agent never ran. The reconcile loop retries until DispatchRetryWindow runs out.
			if rerr := uc.recordDispatchError(ctx, req, task, err); rerr != nil {
				return res, rerr
			}
			return res, errors.Join(errs...)
		default:
			return res, fmt.Errorf("execute task %s: %w", task.ID, err)
		}
		if shared == "" {
			break // the first task of the Plan creates the worktree; the others wait for its started event
		}
	}
	return res, errors.Join(errs...)
}

func (uc *AdvanceExecution) openGate(ctx context.Context, req domain.Request, gate domain.GateRequirement, actorID string) error {
	if uc.Approvals == nil {
		return errors.New("a type-policy gate needs the approval service, which is disabled")
	}
	_, err := uc.Approvals.Execute(ctx, OpenApprovalInput{RequestID: req.ID, SubjectType: gate.SubjectType, SubjectID: gate.SubjectID, RequestedBy: actorID})
	return err
}

func (uc *AdvanceExecution) recordDispatchError(ctx context.Context, req domain.Request, task domain.TaskView, cause error) error {
	code := "TASK_EXECUTE_FAILED"
	var de *domain.DispatchError
	if errors.As(cause, &de) {
		code = de.Code
	}
	tenantID, _ := tenant.TenantID(ctx)
	_, err := uc.Outcomes.Insert(ctx, domain.TaskRunOutcome{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, TaskID: task.ID, ContainerID: task.ParentID,
		Outcome: domain.OutcomeFailed, Cause: domain.CauseDispatchError, ErrorMessage: code, EventID: uuid.NewString(), OccurredAt: uc.now(),
	})
	uc.log().Warn("task dispatch failed, will retry", slog.String("request_id", req.ID), slog.String("task_id", task.ID), slog.String("code", code))
	return err
}

// sharedWorktree picks the Plan's worktree: the one a task that already ran used, else any task's.
func sharedWorktree(tree domain.ExecutionTree) string {
	fallback := ""
	for _, l := range tree.Leaves {
		if l.WorktreeID == "" {
			continue
		}
		switch l.Status {
		case domain.TaskStatusDone, domain.TaskStatusReview, domain.TaskStatusInProgress:
			return l.WorktreeID
		}
		if fallback == "" {
			fallback = l.WorktreeID
		}
	}
	return fallback
}

func startedPhases(ctx context.Context, repo PhaseStartRepository, requestID string) (map[string]bool, error) {
	starts, err := repo.ListByRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(starts))
	for _, s := range starts {
		out[s.PhaseTaskID] = true
	}
	return out, nil
}
