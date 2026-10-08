package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func advance(t *testing.T, r *exRig, req domain.Request, container string) AdvanceResult {
	t.Helper()
	res, err := r.advance.Execute(lcCtx(), AdvanceInput{RequestID: req.ID, ContainerID: container, ActorID: "approver-1"})
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	return res
}

func TestAdvanceExecution_SelectsOnlyOpen(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	plan := r.plan(req)
	open := r.leaf(req, plan, "open", domain.TaskStatusOpen)
	r.leaf(req, plan, "blocked", domain.TaskStatusBlocked)
	r.leaf(req, plan, "review", domain.TaskStatusReview)
	r.leaf(req, plan, "done", domain.TaskStatusDone)
	r.leaf(req, plan, "cancelled", domain.TaskStatusCancelled)

	res := advance(t, r, req, "")
	if len(res.Dispatched) != 1 || res.Dispatched[0] != open.ID {
		t.Fatalf("only the open task may run, got %v", res.Dispatched)
	}
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("calls: %v", r.tasks.calls)
	}
}

func TestAdvanceExecution_RespectsMaxParallel(t *testing.T) {
	r := newExRig(t, func(s *ExecutionSettings) { s.MaxParallelTasks = 2 })
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	plan := r.plan(req)
	running := r.leaf(req, plan, "running", domain.TaskStatusInProgress)
	r.tasks.tasks[running.ID] = withStatus(running, domain.TaskStatusInProgress)
	for _, id := range []string{"a", "b", "c"} {
		task := r.leaf(req, plan, id, domain.TaskStatusOpen)
		_ = task
	}
	// A worktree already exists, so the round may dispatch more than the first task.
	for id, task := range r.tasks.tasks {
		task.WorktreeID = "wt-0"
		r.tasks.tasks[id] = task
	}
	res := advance(t, r, req, "")
	if len(res.Dispatched) != 1 {
		t.Fatalf("2 slots minus 1 running leaves 1 dispatch, got %v", res.Dispatched)
	}
	if again := advance(t, r, req, ""); len(again.Dispatched) != 0 {
		t.Fatalf("both slots taken, got %v", again.Dispatched)
	}
}

func TestAdvanceExecution_AlreadyInProgressIsSuccess(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.tasks.executeErrs[task.ID] = []error{domain.ErrTaskAlreadyRunning}
	res, err := r.advance.Execute(lcCtx(), AdvanceInput{RequestID: req.ID, ActorID: "approver-1"})
	if err != nil || len(res.Dispatched) != 1 {
		t.Fatalf("ALREADY_IN_PROGRESS is success: %v %v", res, err)
	}
}

func TestAdvanceExecution_TransientErrorNotCountedAsAttempt(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.tasks.executeErrs[task.ID] = []error{&domain.DispatchError{Code: "TASK_EXECUTE_NO_CONNECTION", Err: domain.ErrTaskDispatchTransient}}

	res, err := r.advance.Execute(lcCtx(), AdvanceInput{RequestID: req.ID, ActorID: "approver-1"})
	if err != nil || len(res.Dispatched) != 0 {
		t.Fatalf("a transient failure is not an error and dispatches nothing: %v %v", res, err)
	}
	if n, _ := r.outcomes.CountFailed(lcCtx(), task.ID); n != 0 {
		t.Fatalf("a dispatch error must not use up an attempt, CountFailed=%d", n)
	}
	last, ok, _ := r.outcomes.LatestDispatchError(lcCtx(), task.ID)
	if !ok || last.ErrorMessage != "TASK_EXECUTE_NO_CONNECTION" {
		t.Fatalf("dispatch error not recorded: %+v", last)
	}
	advance(t, r, req, "")
	if len(r.tasks.executed) != 1 || !strings.HasSuffix(r.tasks.executed[0], ":1") {
		t.Fatalf("the retry is still attempt 1: %v", r.tasks.executed)
	}
}

func TestAdvanceExecution_ShareWorktreeAfterFirst(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	plan := r.plan(req)
	first := r.leaf(req, plan, "first", domain.TaskStatusOpen)
	second := r.leaf(req, plan, "second", domain.TaskStatusOpen)

	advance(t, r, req, "")
	wt := r.tasks.get(first.ID).WorktreeID
	if wt == "" || r.tasks.get(second.ID).WorktreeID != "" {
		t.Fatalf("the first task creates the worktree, the second must wait: %q %q", wt, r.tasks.get(second.ID).WorktreeID)
	}
	r.finishRun(req, first, true, "")
	// The review->done step happened in finishRun's event; the second task now runs in the same worktree.
	if got := r.tasks.get(second.ID).WorktreeID; got != wt {
		t.Fatalf("second task worktree %q, want %q (calls %v)", got, wt, r.tasks.calls)
	}
	if r.tasks.get(second.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("second task should be running: %v", r.tasks.calls)
	}
	setIdx, execIdx := -1, -1
	for i, c := range r.tasks.calls {
		if c == "worktree:"+second.ID+":"+wt {
			setIdx = i
		}
		if c == "execute:"+second.ID {
			execIdx = i
		}
	}
	if setIdx < 0 || execIdx < 0 || setIdx > execIdx {
		t.Fatalf("worktree must be set before Execute: %v", r.tasks.calls)
	}
}

func TestAdvanceExecution_GateBlocks_OpensApprovalAndStops(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	gated := r.leaf(req, plan, "drop table", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)

	for i := 0; i < 3; i++ {
		res := advance(t, r, req, "")
		if len(res.Dispatched) != 0 || len(res.GatesOpened) != 1 {
			t.Fatalf("run %d: %+v", i, res)
		}
	}
	if r.tasks.count("execute:") != 0 {
		t.Fatalf("a gated task must not be dispatched: %v", r.tasks.calls)
	}
	pending := 0
	for _, a := range r.gates.approvals {
		if a.Status == domain.ApprovalStatusPending && a.SubjectType == domain.SubjectPreDeploy && a.SubjectID == gated.ID {
			pending++
		}
	}
	if pending != 1 || r.tasks.get(gated.ID).Status != domain.TaskStatusOpen {
		t.Fatalf("exactly one pending pre_deploy approval and the task stays open, got %d / %s", pending, r.tasks.get(gated.ID).Status)
	}
}

func TestAdvanceExecution_AfterApproval_Dispatches(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	gated := r.leaf(req, r.plan(req), "drop table", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)
	free := r.leaf(req, r.tasks.get(r.tasks.order[0]), "free", domain.TaskStatusOpen)
	advance(t, r, req, "")
	if r.tasks.count("execute:"+gated.ID) != 0 {
		t.Fatal("gated task ran before approval")
	}
	if r.tasks.get(free.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("an ungated sibling runs normally: %v", r.tasks.calls)
	}
	r.approved(req, domain.SubjectPreDeploy, gated.ID)
	r.finishRun(req, free, true, "")
	if r.tasks.get(gated.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("after approval the gated task must be dispatched: %v", r.tasks.calls)
	}
}

func TestAdvanceExecution_Idempotent_RerunDispatchesNothingNew(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	advance(t, r, req, "")
	for i := 0; i < 4; i++ {
		if res := advance(t, r, req, ""); len(res.Dispatched) != 0 {
			t.Fatalf("rerun %d dispatched %v", i, res.Dispatched)
		}
	}
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("Execute must be called once, calls %v", r.tasks.calls)
	}
}

func TestAdvanceExecution_NotExecuting(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusRequestBacklog)
	_, err := r.advance.Execute(lcCtx(), AdvanceInput{RequestID: req.ID})
	if !errorHasCode(err, "REQUEST_NOT_EXECUTING") {
		t.Fatalf("got %v", err)
	}
}

func TestAdvanceExecution_ForbiddenMapsToExecuteForbidden(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.tasks.executeErrs[task.ID] = []error{domain.ErrTaskForbidden}
	_, err := r.advance.Execute(lcCtx(), AdvanceInput{RequestID: req.ID, ActorID: "approver-1"})
	if !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("got %v", err)
	}
}

func TestAdvanceExecution_UsesApproverIdentity(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	advance(t, r, req, "")
	if len(r.tasks.executedBy) != 1 || r.tasks.executedBy[0] != "approver-1" {
		t.Fatalf("Execute must carry the approving user, got %v", r.tasks.executedBy)
	}
}

func TestAdvanceExecution_OnlyStartedPhasesRun(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	t1 := r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.leaf(req, p2, "t2", domain.TaskStatusOpen)

	if res := advance(t, r, req, ""); len(res.Dispatched) != 0 {
		t.Fatalf("no phase started yet, got %v", res.Dispatched)
	}
	_, _ = r.starts.TryStart(lcCtx(), domain.PhaseStart{PhaseTaskID: p1.ID, RequestID: req.ID, StartedBy: "u"})
	res := advance(t, r, req, "")
	if len(res.Dispatched) != 1 || res.Dispatched[0] != t1.ID {
		t.Fatalf("only the started phase runs, got %v", res.Dispatched)
	}
}

func TestAdvanceExecution_OutOfAttemptsIsNotRedispatched(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	for i := 0; i < 2; i++ {
		_, _ = r.outcomes.Insert(lcCtx(), domain.TaskRunOutcome{TaskID: task.ID, Outcome: domain.OutcomeFailed, Cause: domain.CauseExecutionFailed, EventID: string(rune('a' + i))})
	}
	if res := advance(t, r, req, ""); len(res.Dispatched) != 0 {
		t.Fatalf("got %v", res.Dispatched)
	}
}

func TestAdvanceExecution_OtherErrorsAreReturned(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	r.tasks.executeErrs[task.ID] = []error{errors.New("prompt unsupported")}
	if _, err := r.advance.Execute(lcCtx(), AdvanceInput{RequestID: req.ID}); err == nil {
		t.Fatal("an unclassified Execute failure must surface")
	}
}
