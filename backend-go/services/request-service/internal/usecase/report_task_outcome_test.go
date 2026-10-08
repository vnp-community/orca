package usecase

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func countSubject(r *exRig, subject string) int {
	n := 0
	for _, s := range r.store.subjects() {
		if s == subject {
			n++
		}
	}
	return n
}

// runningTask starts the only task of a one-task Plan and returns it.
func runningTask(t *testing.T, r *exRig, typ domain.RequestType) (domain.Request, domain.TaskView, domain.TaskView) {
	t.Helper()
	req := r.request(typ, domain.RequestSizeS, domain.RequestStatusExecuting)
	plan := r.plan(req)
	task := r.leaf(req, plan, "build the thing", domain.TaskStatusOpen)
	advance(t, r, req, "")
	return req, plan, r.tasks.get(task.ID)
}

func TestReportTaskOutcome_Duplicate_ProcessedOnce(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypeTask)
	r.tasks.mu.Lock()
	r.tasks.tasks[task.ID] = withStatus(task, domain.TaskStatusOpen)
	r.tasks.mu.Unlock()
	in := ReportTaskOutcomeInput{EventID: uuid.NewString(), RequestID: req.ID, TaskID: task.ID, TaskType: "task", ParentID: task.ParentID, Cause: domain.CauseExecutionFailed, NewStatus: domain.TaskStatusOpen, ErrorMessage: "crash"}
	for i := 0; i < 3; i++ {
		if err := r.report.Execute(lcCtx(), in); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := r.outcomes.CountFailed(lcCtx(), task.ID); n != 1 {
		t.Fatalf("one event counts once, got %d", n)
	}
	if r.tasks.count("execute:") != 2 {
		t.Fatalf("one run plus one retry, got calls %v", r.tasks.calls)
	}
}

func TestReportTaskOutcome_OrphanRequest_Ignored(t *testing.T) {
	r := newExRig(t)
	err := r.report.Execute(lcCtx(), ReportTaskOutcomeInput{EventID: uuid.NewString(), RequestID: uuid.NewString(), TaskID: "t", TaskType: "task", Cause: domain.CauseExecutionFailed, NewStatus: "open"})
	if err != nil || len(r.outcomes.rows) != 0 {
		t.Fatalf("an orphan request_id is skipped silently: %v %v", err, r.outcomes.rows)
	}
}

func TestReportTaskOutcome_OutsideTable_Ignored(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypeTask)
	if err := r.event(req, task, "user_update", "review", ""); err != nil || len(r.outcomes.rows) != 0 {
		t.Fatalf("an event outside the table leaves no row: %v %v", err, r.outcomes.rows)
	}
}

func TestReportTaskOutcome_InvalidInput(t *testing.T) {
	r := newExRig(t)
	if err := r.report.Execute(lcCtx(), ReportTaskOutcomeInput{RequestID: "r"}); !errorHasCode(err, "REQUEST_OUTCOME_INVALID") {
		t.Fatalf("got %v", err)
	}
}

func TestReportTaskOutcome_RequestNotExecuting_OnlyRecords(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusRequestBacklog)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusReview)
	if err := r.event(req, task, domain.CauseExecutionCompleted, domain.TaskStatusReview, ""); err != nil {
		t.Fatal(err)
	}
	if len(r.outcomes.rows) != 1 || r.outcomes.rows[0].Outcome != domain.OutcomeSucceeded {
		t.Fatalf("outcome should be recorded: %+v", r.outcomes.rows)
	}
	if len(r.tasks.calls) != 0 {
		t.Fatalf("a Request that left executing must not drive task-service: %v", r.tasks.calls)
	}
}

func TestReportTaskOutcome_SuccessAutoCompletes(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	plan := r.plan(req)
	first := r.leaf(req, plan, "first", domain.TaskStatusOpen)
	second := r.leaf(req, plan, "second", domain.TaskStatusBlocked)
	r.dependsOn(second, first)
	advance(t, r, req, "")
	r.finishRun(req, first, true, "")
	if r.tasks.get(first.ID).Status != domain.TaskStatusDone {
		t.Fatalf("a task in review is set to done: %v", r.tasks.calls)
	}
	if r.tasks.get(second.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("the dependent unblocks and runs: %v", r.tasks.calls)
	}
}

func TestReportTaskOutcome_SuccessManualWhenFlagOff(t *testing.T) {
	r := newExRig(t, func(s *ExecutionSettings) { s.AutoCompleteTasks = false })
	req, _, task := runningTask(t, r, domain.RequestTypeTask)
	r.finishRun(req, task, true, "")
	if r.tasks.get(task.ID).Status != domain.TaskStatusReview || r.tasks.count("status:") != 0 {
		t.Fatalf("with the flag off a person sets done: %v", r.tasks.calls)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("request must keep waiting, got %s", got.Status)
	}
}

func TestReportTaskOutcome_FailureRetriesThenBacklog(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypeTask)

	r.finishRun(req, task, false, "agent crashed")
	if r.tasks.get(task.ID).Status != domain.TaskStatusInProgress || len(r.tasks.executed) != 2 || !strings.HasSuffix(r.tasks.executed[1], ":2") {
		t.Fatalf("attempt 1 failed: the task must run again as attempt 2: %v", r.tasks.executed)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("still executing after the first failure, got %s", got.Status)
	}
	r.finishRun(req, task, false, "agent crashed again")
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStageTask || got.ReturnedCategory != domain.ReturnCategoryOther {
		t.Fatalf("out of attempts the request returns to the backlog: %+v", got)
	}
	for _, want := range []string{"build the thing", "2", "agent crashed again"} {
		if !strings.Contains(got.ReturnReason, want) {
			t.Errorf("reason %q lacks %q", got.ReturnReason, want)
		}
	}
	if len(r.tasks.executed) != 2 {
		t.Fatalf("no third run: %v", r.tasks.executed)
	}
}

func TestReportTaskOutcome_PhaseDone_OpensNextPhaseApprovalOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1 := r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.leaf(req, p2, "t2", domain.TaskStatusOpen)
	r.approved(req, domain.SubjectPhase, p1.ID)
	if _, err := startPhase(r, req, p1.ID); err != nil {
		t.Fatal(err)
	}
	r.finishRun(req, t1, true, "")
	if r.tasks.get(p1.ID).Status != domain.TaskStatusDone {
		t.Fatalf("phase 1 should be derived done: %v", r.tasks.calls)
	}
	// task-service reports the derived completion of phase 1 (twice: at-least-once delivery, then a second event id)
	for i := 0; i < 2; i++ {
		if err := r.event(req, r.tasks.get(p1.ID), domain.CauseDerived, domain.TaskStatusDone, ""); err != nil {
			t.Fatal(err)
		}
	}
	if n := countSubject(r, domain.SubjectPhaseCompleted); n != 1 {
		t.Fatalf("phase.completed once, got %d", n)
	}
	pending := 0
	for _, a := range r.gates.approvals {
		if a.SubjectType == domain.SubjectPhase && a.SubjectID == p2.ID && a.Status == domain.ApprovalStatusPending {
			pending++
		}
	}
	if pending != 1 {
		t.Fatalf("exactly one approval for the next phase, got %d (opener %+v)", pending, r.opener.calls)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("the request waits in executing for the next phase, got %s", got.Status)
	}
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("the next phase must not run before it is started: %v", r.tasks.calls)
	}
}

func TestReportTaskOutcome_LastPhaseDone_CompletesRequest(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "only")
	task := r.leaf(req, ph, "t", domain.TaskStatusOpen)
	if _, err := startPhase(r, req, ph.ID); err != nil {
		t.Fatal(err)
	}
	r.finishRun(req, task, true, "")
	got := r.reload(req)
	if got.Status != domain.RequestStatusCompleted {
		t.Fatalf("every phase done completes the request, got %s", got.Status)
	}
	if countSubject(r, domain.SubjectRequestCompleted) != 1 {
		t.Fatalf("request.completed expected: %v", r.store.subjects())
	}
	if !r.outcomes.has(domain.OutcomePlanDone) {
		t.Fatal("plan_done must be recorded")
	}
}

func TestReportTaskOutcome_NoPhases_AllLeavesDone_Completes(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypeTask)
	r.finishRun(req, task, true, "")
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("got %s", got.Status)
	}
}

func TestReportTaskOutcome_CompletionCheckMissing_Waits(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypePerformance)
	_, _ = r.checks.Append(lcCtx(), domain.RequestCheck{RequestID: req.ID, Kind: domain.CheckPerfBaseline, Status: domain.CheckStatusPassed, Source: domain.CheckSourceManual, Metrics: []byte(`{"metrics":[{"name":"p95","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":20}],"method":"wrk"}`)})
	r.finishRun(req, task, true, "")
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("without perf_after the request waits, got %s", got.Status)
	}
	_, _ = r.checks.Append(lcCtx(), domain.RequestCheck{RequestID: req.ID, Kind: domain.CheckPerfAfter, Status: domain.CheckStatusPassed, Source: domain.CheckSourceAgent, Metrics: []byte(`{"metrics":[{"name":"p95","value":150}]}`)})
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("once the numbers arrive the request completes, got %s", got.Status)
	}
}

func TestReportTaskOutcome_CompletionCheckFailed_ReturnsBacklog(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypePerformance)
	_, _ = r.checks.Append(lcCtx(), domain.RequestCheck{RequestID: req.ID, Kind: domain.CheckPerfBaseline, Status: domain.CheckStatusPassed, Source: domain.CheckSourceManual, Metrics: []byte(`{"metrics":[{"name":"p95","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":20}],"method":"wrk"}`)})
	// The agent says "passed" but the numbers fall short: Orca recomputes and refuses.
	_, _ = r.checks.Append(lcCtx(), domain.RequestCheck{RequestID: req.ID, Kind: domain.CheckPerfAfter, Status: domain.CheckStatusPassed, Source: domain.CheckSourceAgent, Metrics: []byte(`{"metrics":[{"name":"p95","value":190}]}`)})
	r.finishRun(req, task, true, "")
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryInfeasible || got.ReturnedFromStage != domain.ReturnStageTask {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(got.ReturnReason, "p95") {
		t.Fatalf("the reason names the metric: %q", got.ReturnReason)
	}
}

func TestReportTaskOutcome_AllChildrenCancelled_ReturnsBacklog(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	a := r.leaf(req, ph, "a", domain.TaskStatusCancelled)
	r.leaf(req, ph, "b", domain.TaskStatusCancelled)
	_, _ = r.starts.TryStart(lcCtx(), domain.PhaseStart{PhaseTaskID: ph.ID, RequestID: req.ID, StartedBy: "u"})
	if err := r.event(req, a, "user_update", domain.TaskStatusCancelled, ""); err != nil {
		t.Fatal(err)
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStagePhase || got.ReturnedCategory != domain.ReturnCategoryOther {
		t.Fatalf("got %+v", got)
	}
}

func TestReportTaskOutcome_OpsStepFails_BacklogMentionsRollbackTask(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	step := r.leaf(req, plan, "drop the column", domain.TaskStatusOpen)
	undo := r.leaf(req, plan, "restore the column", domain.TaskStatusBlocked, domain.PolicyLabelRollback)
	r.dependsOn(undo, step)
	advance(t, r, req, "")
	r.finishRun(req, step, false, "ddl failed")
	r.finishRun(req, r.tasks.get(step.ID), false, "ddl failed again")
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || !strings.Contains(got.ReturnReason, "restore the column") {
		t.Fatalf("the reason must name the rollback task, got %+v", got)
	}
	if r.tasks.get(undo.ID).Status != domain.TaskStatusBlocked {
		t.Fatal("rollback never runs by itself")
	}
}

func TestReportTaskOutcome_DispatchRetryWindowExpires_BacklogBlockedDependency(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.tasks.executeErrs[task.ID] = []error{
		&domain.DispatchError{Code: "TASK_EXECUTE_NO_CONNECTION", Err: domain.ErrTaskDispatchTransient},
		&domain.DispatchError{Code: "TASK_EXECUTE_NO_CONNECTION", Err: domain.ErrTaskDispatchTransient},
	}
	advance(t, r, req, "")
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("inside the window the request keeps retrying, got %s", got.Status)
	}
	r.clock = r.clock.Add(16 * 60 * 1e9) // 16 minutes
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryBlockedDependency || !strings.Contains(got.ReturnReason, "TASK_EXECUTE_NO_CONNECTION") {
		t.Fatalf("got %+v", got)
	}
}

func TestReportTaskOutcome_HotfixCompletion_RunsFollowUpsOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, domain.TaskView{}, "fix login", domain.TaskStatusOpen)
	advance(t, r, req, "")
	r.finishRun(req, task, true, "")
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("a hotfix with no plan container completes when its task is done, got %s", got.Status)
	}
	if len(r.follow.calls) != 1 || len(r.follow.calls[0]) != 2 {
		t.Fatalf("OnCompleted must hand over two follow-ups once: %+v", r.follow.calls)
	}
	// A repeated evaluation of the already-completed request does nothing more.
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if len(r.follow.calls) != 1 {
		t.Fatalf("follow-ups ran again: %+v", r.follow.calls)
	}
}

func TestReportTaskOutcome_StartedEventEvaluatesOnlyWithParallelism(t *testing.T) {
	r := newExRig(t)
	req, _, task := runningTask(t, r, domain.RequestTypeTask)
	before := r.tasks.listCalls
	if err := r.event(req, task, domain.CauseExecuteClaim, domain.TaskStatusInProgress, ""); err != nil {
		t.Fatal(err)
	}
	if r.tasks.listCalls != before {
		t.Fatal("with one slot a started event needs no re-evaluation")
	}
	r2 := newExRig(t, func(s *ExecutionSettings) { s.MaxParallelTasks = 2 })
	req2, _, task2 := runningTask(t, r2, domain.RequestTypeTask)
	before = r2.tasks.listCalls
	if err := r2.event(req2, task2, domain.CauseExecuteClaim, domain.TaskStatusInProgress, ""); err != nil {
		t.Fatal(err)
	}
	if r2.tasks.listCalls == before {
		t.Fatal("with parallelism a started event releases the waiting tasks")
	}
}

func (f *exOutcomes) has(o domain.Outcome) bool { return f.count(o) > 0 }
