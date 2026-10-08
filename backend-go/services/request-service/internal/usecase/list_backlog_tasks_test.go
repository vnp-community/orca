package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestBacklogTasks_OpenUnderUnapprovedPlan_InTaskNotExecute(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeBug, domain.RequestSizeS, domain.RequestStatusAwaitingPlanApproval, 1)
	plan := b.plan(req)
	task := b.leaf(req, plan, "fix", domain.TaskStatusOpen)

	taskView := b.view(domain.BacklogViewTask)
	if ids := groupIDs(taskView.TaskGroups); len(ids) != 1 || ids[0] != task.ID {
		t.Fatalf("an unapproved task is in TASK: %v", ids)
	}
	if g := taskView.TaskGroups[0]; g.GateStatus != domain.GateStatusNone || g.PlanID != plan.ID || g.PhaseID != "" || g.TotalTasks != 1 {
		t.Fatalf("group: %+v", g)
	}
	if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 0 {
		t.Fatalf("not in EXECUTE yet: %v", ids)
	}

	// The plan is approved and execution starts: the task moves, it does not duplicate.
	b.approved(req, domain.SubjectPlan, plan.ID)
	b.setStatus(req, domain.RequestStatusExecuting)
	if ids := groupIDs(b.view(domain.BacklogViewTask).TaskGroups); len(ids) != 0 {
		t.Fatalf("an approved task leaves TASK: %v", ids)
	}
	exec := b.view(domain.BacklogViewExecute)
	if ids := groupIDs(exec.ExecuteGroups); len(ids) != 1 || ids[0] != task.ID || exec.ExecuteGroups[0].GateStatus != domain.GateStatusApproved {
		t.Fatalf("an approved task is in EXECUTE: %+v", exec.ExecuteGroups)
	}
}

func (b *backlogRig) setStatus(req domain.Request, status domain.RequestStatus) {
	for i, r := range b.reader.reqs {
		if r.ID == req.ID {
			r.Status = status
			b.reader.reqs[i] = r
		}
	}
}

func TestBacklogTasks_ChangeRequest_PhaseNotApproved_StaysInTask(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	ph := b.phase(req, plan, "p1")
	task := b.leaf(req, ph, "t", domain.TaskStatusOpen)
	b.approved(req, domain.SubjectPlan, plan.ID)

	out := b.view(domain.BacklogViewTask)
	if ids := groupIDs(out.TaskGroups); len(ids) != 1 || out.TaskGroups[0].PhaseID != ph.ID || out.TaskGroups[0].PhaseTitle != "p1" {
		t.Fatalf("the plan is approved but the phase is not: still TASK: %+v", out.TaskGroups)
	}
	if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 0 {
		t.Fatalf("not executable: %v", ids)
	}
	b.approved(req, domain.SubjectPhase, ph.ID)
	if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 1 || ids[0] != task.ID {
		t.Fatalf("phase approved: EXECUTE, got %v", ids)
	}
}

func TestBacklogTasks_BugSizeL_NoPhaseYet_InTask(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	b.leaf(req, plan, "t", domain.TaskStatusOpen)
	b.approved(req, domain.SubjectPlan, plan.ID)
	if ids := groupIDs(b.view(domain.BacklogViewTask).TaskGroups); len(ids) != 1 {
		t.Fatalf("a size L bug without phases waits in TASK even though the plan is approved: %v", ids)
	}
	if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 0 {
		t.Fatalf("must not be executable: %v", ids)
	}
}

func TestBacklogTasks_BugSizeL_AfterSplit_PlanApproved_Executes(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	ph := b.phase(req, plan, "p1")
	task := b.leaf(req, ph, "t", domain.TaskStatusOpen)
	b.approved(req, domain.SubjectPlan, plan.ID)
	if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 1 || ids[0] != task.ID {
		t.Fatalf("got %v", ids)
	}
}

func TestBacklogTasks_TaskDocsApprovedPlanShell_InExecute(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeTask, domain.RequestTypeDocs} {
		b := newBacklogRig(t)
		req := b.seed(typ, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
		plan := b.plan(req)
		task := b.leaf(req, plan, "t", domain.TaskStatusOpen)
		b.approved(req, domain.SubjectTaskList, plan.ID)
		if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 1 || ids[0] != task.ID {
			t.Fatalf("%s: got %v", typ, ids)
		}
	}
}

func TestBacklogTasks_HotfixAfterPreDeploy_InExecute(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	fix := b.leaf(req, domain.TaskView{}, "fix", domain.TaskStatusOpen)
	if ids := groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups); len(ids) != 0 {
		t.Fatalf("before pre_deploy: %v", ids)
	}
	b.approved(req, domain.SubjectPreDeploy, fix.ID)
	out := b.view(domain.BacklogViewExecute)
	if ids := groupIDs(out.ExecuteGroups); len(ids) != 1 || ids[0] != fix.ID || out.ExecuteGroups[0].PlanID != "" {
		t.Fatalf("got %+v", out.ExecuteGroups)
	}
}

func TestBacklogTasks_BlockedTask_HasBlockedBy(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	first := b.leaf(req, plan, "first", domain.TaskStatusOpen)
	second := b.leaf(req, plan, "second", domain.TaskStatusBlocked)
	b.dependsOn(second, first)
	b.approved(req, domain.SubjectTaskList, plan.ID)
	rows := b.view(domain.BacklogViewExecute).ExecuteGroups[0].Tasks
	var blocked BacklogTaskRow
	for _, r := range rows {
		if r.Task.ID == second.ID {
			blocked = r
		}
	}
	if len(blocked.BlockedByTaskIDs) != 1 || blocked.BlockedByTaskIDs[0] != first.ID {
		t.Fatalf("blocked_by: %+v", blocked)
	}
}

func TestBacklogTasks_FailedLink_ShowsAttemptsAndError(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	// After a failed run the task is back to open; a second one sits in review with a failed latest link.
	failed := b.leaf(req, plan, "failed", domain.TaskStatusOpen)
	odd := b.leaf(req, plan, "odd", domain.TaskStatusReview)
	b.leaf(req, plan, "plain review", domain.TaskStatusReview)
	b.approved(req, domain.SubjectTaskList, plan.ID)
	b.tasks.states = map[string]ExecutionStateView{
		failed.ID: {LastEngine: "direct_agent", LastLinkStatus: "failed", FailedAttempts: 2},
		odd.ID:    {LastEngine: "direct_agent", LastLinkStatus: "failed", FailedAttempts: 1},
	}
	_, _ = b.outcomes.Insert(backlogAdminCtx(), domain.TaskRunOutcome{TaskID: failed.ID, Outcome: domain.OutcomeFailed, Cause: domain.CauseExecutionFailed, ErrorMessage: "agent crashed", EventID: "e1"})
	got := map[string]BacklogTaskRow{}
	for _, g := range b.view(domain.BacklogViewExecute).ExecuteGroups {
		for _, r := range g.Tasks {
			got[r.Task.ID] = r
		}
	}
	if len(got) != 2 {
		t.Fatalf("open+failed link and review+failed link show; a plain review does not: %v", got)
	}
	row := got[failed.ID]
	if row.FailedAttempts != 2 || row.LastError != "agent crashed" || row.LastEngine != "direct_agent" || row.LastLinkStatus != "failed" {
		t.Fatalf("row: %+v", row)
	}
}

func TestBacklogTasks_DoneAndCancelledNeverShown(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	b.leaf(req, plan, "done", domain.TaskStatusDone)
	b.leaf(req, plan, "cancelled", domain.TaskStatusCancelled)
	b.approved(req, domain.SubjectTaskList, plan.ID)
	b.tasks.states = map[string]ExecutionStateView{}
	for id := range b.tasks.tasks {
		b.tasks.states[id] = ExecutionStateView{LastLinkStatus: "failed"}
	}
	for _, v := range []domain.BacklogView{domain.BacklogViewTask, domain.BacklogViewExecute} {
		out := b.view(v)
		if ids := groupIDs(append(out.TaskGroups, out.ExecuteGroups...)); len(ids) != 0 {
			t.Fatalf("view %d shows finished tasks: %v", v, ids)
		}
	}
}

func TestBacklogTasks_CallCounts_OneListTasksOneStatesOneApprovals(t *testing.T) {
	b := newBacklogRig(t)
	for i := 0; i < 20; i++ {
		req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, i)
		plan := b.plan(req)
		b.leaf(req, plan, "a", domain.TaskStatusOpen)
		b.leaf(req, plan, "b", domain.TaskStatusBlocked)
		b.approved(req, domain.SubjectTaskList, plan.ID)
	}
	b.view(domain.BacklogViewExecute, func(in *ListBacklogInput) { in.PageSize = 20 })
	if b.tasks.listCalls != 1 || b.tasks.statesCalls != 1 || b.gates.calls != 1 || b.reader.pages != 1 {
		t.Fatalf("20 requests must cost one ListTasks (%d), one ListExecutionStates (%d), one approvals query (%d), one request page (%d)",
			b.tasks.listCalls, b.tasks.statesCalls, b.gates.calls, b.reader.pages)
	}
	b.tasks.listCalls, b.tasks.statesCalls, b.gates.calls = 0, 0, 0
	b.view(domain.BacklogViewTask, func(in *ListBacklogInput) { in.PageSize = 20 })
	if b.tasks.listCalls != 1 || b.tasks.statesCalls != 0 || b.gates.calls != 1 {
		t.Fatalf("task view: lists=%d states=%d approvals=%d", b.tasks.listCalls, b.tasks.statesCalls, b.gates.calls)
	}
}

func TestBacklogTasks_TaskServiceDown_ReturnsUnavailable_NoPartial(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	b.leaf(req, b.plan(req), "a", domain.TaskStatusOpen)
	b.tasks.listErr = fmt.Errorf("list: %w", domain.ErrTaskServiceDown)
	for _, v := range []domain.BacklogView{domain.BacklogViewTask, domain.BacklogViewExecute} {
		out, err := b.uc.Execute(backlogAdminCtx(), ListBacklogInput{View: v})
		if !errorHasCode(err, "REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE") || len(out.TaskGroups)+len(out.ExecuteGroups) != 0 {
			t.Fatalf("view %d: %v %+v", v, err, out)
		}
	}
	// The request view keeps working while task-service is away.
	b.reader.reqs = append(b.reader.reqs, b.store.seed(func(r *domain.Request) {
		r.Status, r.ReturnedCategory = domain.RequestStatusRequestBacklog, domain.ReturnCategoryOther
	}))
	if _, err := b.uc.Execute(backlogAdminCtx(), ListBacklogInput{View: domain.BacklogViewRequest}); err != nil {
		t.Fatalf("request view: %v", err)
	}
}

func TestBacklogTasks_StatesOutageIsUnavailableToo(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	b.leaf(req, plan, "a", domain.TaskStatusOpen)
	b.approved(req, domain.SubjectTaskList, plan.ID)
	flaky := &exStatesDown{exTasks: b.tasks}
	b.uc.Tasks.Tasks = flaky
	if _, err := b.uc.Execute(backlogAdminCtx(), ListBacklogInput{View: domain.BacklogViewExecute}); !errorHasCode(err, "REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE") {
		t.Fatalf("got %v", err)
	}
	if _, _, err := (&ListBacklogTasks{}).Execute(backlogAdminCtx(), ListBacklogTasksInput{View: domain.BacklogViewRequest}); !errorHasCode(err, "REQUEST_BACKLOG_INVALID_VIEW") {
		t.Fatalf("the request view is not this use case's: %v", err)
	}
}

type exStatesDown struct{ *exTasks }

func (f *exStatesDown) ListExecutionStates(context.Context, []string) (map[string]ExecutionStateView, error) {
	return nil, domain.ErrTaskServiceDown
}

func TestBacklogTasks_PlanWith150Tasks_CappedAt100(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	for i := 0; i < 150; i++ {
		b.leaf(req, plan, fmt.Sprintf("t%03d", i), domain.TaskStatusOpen)
	}
	b.approved(req, domain.SubjectTaskList, plan.ID)
	out := b.view(domain.BacklogViewExecute)
	if got := len(groupIDs(out.ExecuteGroups)); got != 100 {
		t.Fatalf("a plan shows at most 100 tasks, got %d", got)
	}
	if out.ExecuteGroups[0].TotalTasks != 150 {
		t.Fatalf("total_tasks still tells the truth: %d", out.ExecuteGroups[0].TotalTasks)
	}
}

func TestBacklogTasks_Filters(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting, 1)
	plan := b.plan(req)
	p1, p2 := b.phase(req, plan, "p1"), b.phase(req, plan, "p2")
	a := b.leaf(req, p1, "a", domain.TaskStatusOpen)
	c := b.leaf(req, p2, "c", domain.TaskStatusOpen)
	a.AssigneeID = "alice"
	b.tasks.tasks[a.ID] = a
	b.approved(req, domain.SubjectPlan, plan.ID)

	if ids := groupIDs(b.view(domain.BacklogViewTask, func(in *ListBacklogInput) { in.PhaseTaskID = p2.ID }).TaskGroups); len(ids) != 1 || ids[0] != c.ID {
		t.Fatalf("phase filter: %v", ids)
	}
	if ids := groupIDs(b.view(domain.BacklogViewTask, func(in *ListBacklogInput) { in.AssigneeID = "alice" }).TaskGroups); len(ids) != 1 || ids[0] != a.ID {
		t.Fatalf("assignee filter: %v", ids)
	}
	if ids := groupIDs(b.view(domain.BacklogViewTask, func(in *ListBacklogInput) { in.PlanTaskID = "other-plan" }).TaskGroups); len(ids) != 0 {
		t.Fatalf("plan filter: %v", ids)
	}
	if ids := groupIDs(b.view(domain.BacklogViewTask, func(in *ListBacklogInput) { in.RequestID = req.ID }).TaskGroups); len(ids) != 2 {
		t.Fatalf("request filter: %v", ids)
	}
}

func TestBacklogTasks_TaskViewAlsoCoversAwaitingPlanApproval_ExecuteDoesNot(t *testing.T) {
	b := newBacklogRig(t)
	req := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusAwaitingPlanApproval, 1)
	plan := b.plan(req)
	b.leaf(req, plan, "a", domain.TaskStatusOpen)
	b.approved(req, domain.SubjectTaskList, plan.ID) // even with the gate open, EXECUTE needs an executing request
	if len(groupIDs(b.view(domain.BacklogViewExecute).ExecuteGroups)) != 0 {
		t.Fatal("EXECUTE needs an executing request")
	}
}
