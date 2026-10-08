package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// EvaluateExecution is the state-based half of the feedback loop (CR-REQ-013 2.5.1 step 3 and 4): it reads the
// Request's tasks and does whatever the current state calls for. The outcome consumer and the reconcile loop both
// call it, so a lost event costs only latency. Every step is idempotent.
type EvaluateExecution struct {
	Tasks       TaskClient
	Outcomes    TaskRunOutcomeRepository
	PhaseStarts PhaseStartRepository
	Checks      RequestCheckRepository
	Approvals   ApprovalGateReader
	// Open opens the next Phase's Approval; nil disables it.
	Open       ApprovalOpener
	Advance    *AdvanceExecution
	Actors     ExecutionActors
	Policies   *domain.PolicyRegistry
	Transition RequestTransitioner
	Returner   RequestReturner
	// FollowUps may be nil: policies then still run, their follow-ups are logged and dropped.
	FollowUps FollowUpExecutor
	Tx        TxRunner
	Outbox    OutboxWriter
	Settings  ExecutionSettings
	Log       *slog.Logger
	Now       func() time.Time
}

func (e *EvaluateExecution) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}

func (e *EvaluateExecution) log() *slog.Logger {
	if e.Log != nil {
		return e.Log
	}
	return slog.Default()
}

// Run evaluates one Request; a Request that is not executing is ignored.
func (e *EvaluateExecution) Run(ctx context.Context, req domain.Request) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	defer e.Advance.lockRequest(tenantID, req.ID)()
	if req.Status != domain.RequestStatusExecuting {
		return nil
	}
	tasks, err := e.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return err
	}
	tree := domain.BuildExecutionTree(tasks)
	started, err := startedPhases(ctx, e.PhaseStarts, req.ID)
	if err != nil {
		return err
	}
	settings := e.Settings.normalized()
	flow, _ := domain.FlowFor(req.Type)

	completed, err := e.autoComplete(ctx, req, tree)
	if completed > 0 {
		// Completing a task unblocks its dependents and may finish a Phase; read the result instead of waiting for the events.
		tasks, rerr := e.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
		if rerr != nil {
			return errors.Join(err, rerr)
		}
		tree = domain.BuildExecutionTree(tasks)
	}
	if err != nil {
		return err
	}
	if returned, err := e.returnOnFailure(ctx, req, tree, started, flow, settings); err != nil || returned {
		return err
	}
	if err := e.completePhases(ctx, req, tree, started, flow); err != nil {
		return err
	}
	if tree.ScopeFinished() {
		return e.finish(ctx, req, tree)
	}
	return e.dispatch(ctx, req, tree, started)
}

// autoComplete turns review into done (REQUEST_AUTO_COMPLETE_TASKS): only done unblocks the next task.
func (e *EvaluateExecution) autoComplete(ctx context.Context, req domain.Request, tree domain.ExecutionTree) (int, error) {
	if !e.Settings.AutoCompleteTasks {
		return 0, nil
	}
	var errs []error
	completed := 0
	for _, l := range tree.Leaves {
		if l.Status != domain.TaskStatusReview {
			continue
		}
		actor, err := e.Actors.ActorFor(ctx, req, phaseOf(tree, l))
		if err != nil {
			return completed, err
		}
		if err := e.Tasks.SetStatus(tenant.WithUserID(ctx, actor), l.ID, domain.TaskStatusDone); err != nil {
			errs = append(errs, fmt.Errorf("complete task %s: %w", l.ID, err))
			continue
		}
		completed++
	}
	return completed, errors.Join(errs...)
}

func phaseOf(tree domain.ExecutionTree, task domain.TaskView) string {
	if c, ok := tree.Container(task); ok && c.Type == domain.TaskTypePhase {
		return c.ID
	}
	return ""
}

// returnOnFailure sends the Request back when a task ran out of attempts, could not be dispatched for too long,
// or a whole container was cancelled. returned=true means the caller must stop.
func (e *EvaluateExecution) returnOnFailure(ctx context.Context, req domain.Request, tree domain.ExecutionTree, started map[string]bool,
	flow domain.FlowDefinition, settings ExecutionSettings) (bool, error) {
	var in *ReturnInput
	for _, l := range tree.Leaves {
		if l.Status != domain.TaskStatusOpen || !tree.InScope(l, started) {
			continue
		}
		failed, err := e.Outcomes.CountFailed(ctx, l.ID)
		if err != nil {
			return false, err
		}
		if failed >= settings.MaxTaskAttempts {
			in = e.exhaustedReturn(ctx, req, tree, l, failed, flow)
			break
		}
		if since, ok, err := e.Outcomes.DispatchRetrySince(ctx, l.ID); err != nil {
			return false, err
		} else if ok && e.now().Sub(since) > settings.DispatchRetryWindow {
			code := "TASK_EXECUTE_FAILED"
			if last, found, err := e.Outcomes.LatestDispatchError(ctx, l.ID); err == nil && found && last.ErrorMessage != "" {
				code = last.ErrorMessage
			}
			in = &ReturnInput{Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryBlockedDependency, Reason: "Không chạy được: " + code}
			break
		}
	}
	if in == nil {
		in = e.allCancelledReturn(tree, started, flow, req)
	}
	if in == nil {
		return false, nil
	}
	in.RequestID, in.ActorKind = req.ID, domain.ActorKindSystem
	if _, err := e.Returner.Execute(ctx, *in); err != nil {
		if errorHasCode(err, "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION") {
			e.log().Info("return to backlog waits for a running task", slog.String("request_id", req.ID))
			return true, nil // the reconcile loop tries again once nothing runs
		}
		return false, err
	}
	return true, nil
}

func (e *EvaluateExecution) exhaustedReturn(ctx context.Context, req domain.Request, tree domain.ExecutionTree, task domain.TaskView, failed int, flow domain.FlowDefinition) *ReturnInput {
	lastErr := ""
	if m, err := e.Outcomes.LatestFailed(ctx, []string{task.ID}); err == nil {
		lastErr = m[task.ID].ErrorMessage
	}
	reason := fmt.Sprintf("Task %q lỗi sau %d lần: %s", task.Title, failed, lastErr)
	if hinter, ok := e.Policies.PolicyFor(req.Type).(domain.FailureHinter); ok {
		all := e.refsWithEdges(ctx, tree)
		if hint := hinter.FailureHint(req, task.Ref(), all); hint != "" {
			reason += ". " + hint
		}
	}
	return &ReturnInput{Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryOther, Reason: reason}
}

// refsWithEdges loads depends_on edges (one GetSubtree) so a policy can tell which tasks follow the failed one.
func (e *EvaluateExecution) refsWithEdges(ctx context.Context, tree domain.ExecutionTree) []domain.TaskRef {
	refs := make([]domain.TaskRef, 0, len(tree.Leaves))
	for _, l := range tree.Leaves {
		refs = append(refs, l.Ref())
	}
	if tree.Plan == nil {
		return refs
	}
	sub, err := e.Tasks.GetSubtree(ctx, tree.Plan.ID)
	if err != nil {
		return refs
	}
	deps := map[string][]string{}
	for _, edge := range sub.DependsOn {
		deps[edge.From] = append(deps[edge.From], edge.To)
	}
	for i := range refs {
		refs[i].DependsOn = deps[refs[i].ID]
	}
	return refs
}

func (e *EvaluateExecution) allCancelledReturn(tree domain.ExecutionTree, started map[string]bool, flow domain.FlowDefinition, req domain.Request) *ReturnInput {
	containers := map[string][]domain.TaskView{}
	for _, l := range tree.Leaves {
		if tree.InScope(l, started) {
			containers[l.ParentID] = append(containers[l.ParentID], l)
		}
	}
	for id, leaves := range containers {
		allCancelled := true
		for _, l := range leaves {
			allCancelled = allCancelled && l.Status == domain.TaskStatusCancelled
		}
		if !allCancelled {
			continue
		}
		// Stage "plan" is not valid while executing, so a Plan-level cancel returns from the task stage.
		stage := domain.ReturnStageTask
		if _, isPhase := tree.Phase(id); isPhase && flow.PhasesFor(req.Size) {
			stage = domain.ReturnStagePhase
		}
		return &ReturnInput{Stage: stage, Category: domain.ReturnCategoryOther, Reason: "Toàn bộ task đã huỷ"}
	}
	return nil
}

// completePhases records finished Phases and, when the flow gates Phases, opens the Approval of the next one.
func (e *EvaluateExecution) completePhases(ctx context.Context, req domain.Request, tree domain.ExecutionTree, started map[string]bool, flow domain.FlowDefinition) error {
	finishedAny := false
	for _, p := range tree.Phases {
		if p.Status != domain.TaskStatusDone {
			continue
		}
		finishedAny = true
		count := len(tree.LeavesOf(p.ID))
		if err := e.recordContainerDone(ctx, req, p, domain.OutcomePhaseDone, uuid.NewString(), e.now(), count, true); err != nil {
			return err
		}
	}
	if !finishedAny || e.Open == nil || !flow.HasExecutionGate(domain.GatePhase) {
		return nil
	}
	for _, p := range tree.Phases {
		if started[p.ID] && !taskFinishedStatus(p.Status) {
			return nil // a Phase is still running: the next one waits
		}
	}
	next, ok := tree.NextPhase(started)
	if !ok {
		return nil
	}
	tenantID, _ := tenant.TenantID(ctx)
	approvals, err := e.Approvals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil {
		return err
	}
	if latest := domain.NewApprovalIndex(approvals).GetLatest(domain.SubjectPhase, next.ID); latest != nil &&
		latest.Status != domain.ApprovalStatusExpired && latest.Status != domain.ApprovalStatusCancelled {
		return nil
	}
	actor, err := e.Actors.ActorFor(ctx, req, "")
	if err != nil {
		return err
	}
	_, err = e.Open.Execute(ctx, OpenApprovalInput{RequestID: req.ID, SubjectType: domain.SubjectPhase, SubjectID: next.ID, RequestedBy: actor})
	return err
}

func taskFinishedStatus(s string) bool {
	return s == domain.TaskStatusDone || s == domain.TaskStatusCancelled
}

// recordContainerDone writes the phase_done/plan_done row once and, for a Phase, emits phase.completed in the same
// transaction. emitEvent=false is for rows whose event was already emitted by the caller.
func (e *EvaluateExecution) recordContainerDone(ctx context.Context, req domain.Request, c domain.TaskView, outcome domain.Outcome,
	eventID string, at time.Time, taskCount int, emitEvent bool) error {
	tenantID, _ := tenant.TenantID(ctx)
	return e.Tx.InTx(ctx, func(ctx context.Context) error {
		inserted, err := e.Outcomes.Insert(ctx, domain.TaskRunOutcome{
			ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, TaskID: c.ID, ContainerID: c.ParentID,
			Outcome: outcome, Cause: domain.CauseDerived, EventID: eventID, OccurredAt: at,
		})
		if err != nil || !inserted || outcome != domain.OutcomePhaseDone || !emitEvent {
			return err
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectPhaseCompleted, domain.PhaseCompletedPayload{RequestID: req.ID, PhaseTaskID: c.ID, TaskCount: taskCount})
		if err != nil {
			return err
		}
		return e.Outbox.InsertOutboxEvent(ctx, ev)
	})
}

// finish judges completion with the type policy, then closes the Request or sends it back.
func (e *EvaluateExecution) finish(ctx context.Context, req domain.Request, tree domain.ExecutionTree) error {
	root := domain.TaskView{ID: ""}
	switch {
	case tree.Plan != nil:
		root = *tree.Plan
	case len(tree.Leaves) > 0:
		root = tree.Leaves[0]
	}
	if root.ID != "" {
		if err := e.recordContainerDone(ctx, req, root, domain.OutcomePlanDone, uuid.NewString(), e.now(), len(tree.Leaves), false); err != nil {
			return err
		}
	}
	checks, err := e.Checks.ListByRequest(ctx, req.ID)
	if err != nil {
		return err
	}
	policy := e.Policies.PolicyFor(req.Type)
	verdicts, err := policy.CompletionChecks(req, checks)
	if err != nil {
		return err
	}
	var failed, missing []domain.CheckVerdict
	for _, v := range verdicts {
		switch v.Status {
		case domain.CheckVerdictFailed:
			failed = append(failed, v)
		case domain.CheckVerdictMissing:
			missing = append(missing, v)
		}
	}
	if len(failed) > 0 {
		return e.returnForFailedChecks(ctx, req, failed)
	}
	if len(missing) > 0 {
		e.log().Info("request waits for completion checks", slog.String("request_id", req.ID), slog.Int("missing", len(missing)))
		return nil
	}
	expected := domain.RequestStatusExecuting
	res, err := e.Transition.Execute(ctx, TransitionInput{
		RequestID: req.ID, Trigger: domain.TriggerExecutionFinished, ExpectedFrom: &expected, ActorKind: domain.ActorKindSystem,
	})
	if err != nil {
		return err
	}
	if res.Applied {
		e.runFollowUps(ctx, res.Request, policy)
	}
	return nil
}

func (e *EvaluateExecution) returnForFailedChecks(ctx context.Context, req domain.Request, failed []domain.CheckVerdict) error {
	category := domain.ReturnCategoryOther
	var parts []string
	for _, v := range failed {
		if v.Kind == domain.CheckPerfAfter || v.Kind == domain.CheckPerfBaseline {
			category = domain.ReturnCategoryInfeasible
		}
		parts = append(parts, v.Summary)
	}
	_, err := e.Returner.Execute(ctx, ReturnInput{
		RequestID: req.ID, Stage: domain.ReturnStageTask, Category: category, Reason: strings.Join(parts, "; "), ActorKind: domain.ActorKindSystem,
	})
	if errorHasCode(err, "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION") {
		return nil
	}
	return err
}

// runFollowUps never fails the completion: the Request is already completed and follow-ups are idempotent, so a person can redo them.
func (e *EvaluateExecution) runFollowUps(ctx context.Context, req domain.Request, policy domain.TypePolicy) {
	fus, err := policy.OnCompleted(req)
	if err != nil || len(fus) == 0 {
		if err != nil {
			e.log().Warn("on-completed policy failed", slog.String("request_id", req.ID), slog.Any("error", err))
		}
		return
	}
	if e.FollowUps == nil {
		e.log().Warn("follow-up requests dropped: no executor wired", slog.String("request_id", req.ID), slog.Int("count", len(fus)))
		return
	}
	if err := e.FollowUps.ExecuteFollowUps(ctx, req, fus); err != nil {
		e.log().Warn("follow-up requests failed", slog.String("request_id", req.ID), slog.Any("error", err))
	}
}

// dispatch runs AdvanceExecution for every started, unfinished Phase, or once for a Request without Phases.
func (e *EvaluateExecution) dispatch(ctx context.Context, req domain.Request, tree domain.ExecutionTree, started map[string]bool) error {
	var errs []error
	if !tree.HasPhases() {
		actor, err := e.Actors.ActorFor(ctx, req, "")
		if err != nil {
			return err
		}
		_, err = e.Advance.run(ctx, req, tree, AdvanceInput{RequestID: req.ID, ActorID: actor})
		return err
	}
	for _, p := range tree.Phases {
		if !started[p.ID] || taskFinishedStatus(p.Status) {
			continue
		}
		actor, err := e.Actors.ActorFor(ctx, req, p.ID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := e.Advance.run(ctx, req, tree, AdvanceInput{RequestID: req.ID, ContainerID: p.ID, ActorID: actor}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
