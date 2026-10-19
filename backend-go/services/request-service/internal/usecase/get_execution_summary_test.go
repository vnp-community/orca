package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type summaryClarifications struct{ open *domain.Clarification }

func (c summaryClarifications) GetOpenByRequest(context.Context, string) (*domain.Clarification, error) {
	return c.open, nil
}

type summaryChanges struct{ change *domain.OpenSpecChange }

func (c summaryChanges) GetByRequest(context.Context, string) (domain.OpenSpecChange, bool, error) {
	if c.change == nil {
		return domain.OpenSpecChange{}, false, nil
	}
	return *c.change, true, nil
}

func (r *exRig) summaryUC(auth ApprovalAuthorizer) *GetExecutionSummary {
	return &GetExecutionSummary{
		Requests: r.store, Tasks: r.tasks, Outcomes: r.outcomes, PhaseStarts: r.starts, Approvals: r.gates, Authorizer: auth, MaxAttempts: 2,
		Engine: &RequestEngineResolver{Settings: staticEngineSettings{&domain.ProjectEngineSettings{Engine: domain.EngineOpenSpec}}},
	}
}

func (r *exRig) outcome(req domain.Request, task domain.TaskView, o domain.Outcome, cause, msg string, at time.Time) {
	r.outcomes.rows = append(r.outcomes.rows, domain.TaskRunOutcome{
		ID: fmt.Sprintf("o-%d", len(r.outcomes.rows)), RequestID: req.ID, TaskID: task.ID, Outcome: o, Cause: cause, ErrorMessage: msg, OccurredAt: at,
	})
}

// The shape of request #1242: 7 Phases, 28 tasks, the last task failed twice and sent the Request back.
func TestExecutionSummary_RequestWithAFailedLastTask(t *testing.T) {
	r := newExRig(t)
	req := r.store.seed(func(q *domain.Request) {
		q.Type, q.Size, q.Status, q.ExecutionMode = domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusRequestBacklog, domain.ExecutionContinuous
		q.ReturnedFromStage = domain.ReturnStageTask
	})
	plan := r.plan(req)
	var failedTask domain.TaskView
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	for p := 1; p <= 7; p++ {
		ph := r.phase(req, plan, fmt.Sprintf("phase %d", p))
		for i := 0; i < 4; i++ {
			status := domain.TaskStatusDone
			if p == 7 && i == 3 {
				status = domain.TaskStatusOpen
			}
			task := r.leaf(req, ph, fmt.Sprintf("task %d.%d", p, i), status)
			if status == domain.TaskStatusOpen {
				failedTask = task
			}
		}
		r.starts.rows = append(r.starts.rows, domain.PhaseStart{RequestID: req.ID, PhaseTaskID: ph.ID, StartedBy: "u", Trigger: domain.PhaseStartAuto})
		r.tasks.mu.Lock()
		if p < 7 {
			r.tasks.setStatusLocked(ph.ID, domain.TaskStatusDone)
		}
		r.tasks.mu.Unlock()
	}
	r.outcome(req, failedTask, domain.OutcomeStarted, "execute_claim", "", at)
	r.outcome(req, failedTask, domain.OutcomeFailed, "execution_failed", "execution failed: agent_defect RESULT_BLOCK_MISSING", at.Add(time.Minute))
	r.outcome(req, failedTask, domain.OutcomeStarted, "execute_claim", "", at.Add(2*time.Minute))
	r.outcome(req, failedTask, domain.OutcomeFailed, "execution_failed", "execution failed: retryable TIMED_OUT", at.Add(27*time.Minute))

	uc := r.summaryUC(stubDecider{})
	sum, err := uc.Execute(lcCtx(), req.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := sum.Progress
	if p.PhasesTotal != 7 || p.PhasesDone != 6 || p.TasksTotal != 28 || p.TasksDone != 27 || p.TasksFailed != 1 || p.TasksRunning != 0 {
		t.Fatalf("progress = %+v", p)
	}
	if sum.Engine != domain.EngineOpenSpec || sum.ExecutionMode != domain.ExecutionContinuous || sum.MaxAttempts != 2 {
		t.Fatalf("engine=%q mode=%q max=%d", sum.Engine, sum.ExecutionMode, sum.MaxAttempts)
	}
	if len(sum.Failed) != 1 {
		t.Fatalf("failed = %+v", sum.Failed)
	}
	f := sum.Failed[0]
	if f.TaskID != failedTask.ID || f.Attempts != 2 || f.MaxAttempts != 2 || f.LastCause != domain.FailureTimedOut || f.Title != "task 7.3" {
		t.Fatalf("failed task = %+v", f)
	}
	if len(sum.NeedsYou) != 1 || sum.NeedsYou[0].Kind != domain.NeedsFailedTask || sum.NeedsYou[0].RefID != failedTask.ID {
		t.Fatalf("needs you = %+v", sum.NeedsYou)
	}
	if sum.StartedAt == nil || !sum.StartedAt.Equal(at) {
		t.Fatalf("started at = %v", sum.StartedAt)
	}
}

func TestExecutionSummary_RunningTaskCarriesItsAttempt(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeBug, domain.RequestSizeM)
	task := r.leaf(req, r.plan(req), "run me", domain.TaskStatusInProgress)
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	r.outcome(req, task, domain.OutcomeStarted, "execute_claim", "", at)
	r.outcome(req, task, domain.OutcomeFailed, "execution_failed", "execution failed: retryable TIMED_OUT", at.Add(time.Minute))
	r.outcome(req, task, domain.OutcomeStarted, "execute_claim", "", at.Add(2*time.Minute))

	sum, err := r.summaryUC(stubDecider{}).Execute(lcCtx(), req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Running) != 1 || sum.Running[0].Attempt != 2 || sum.Running[0].MaxAttempts != 2 || !sum.Running[0].StartedAt.Equal(at.Add(2*time.Minute)) {
		t.Fatalf("running = %+v", sum.Running)
	}
	if sum.Progress.TasksRunning != 1 || sum.Progress.TasksFailed != 0 || len(sum.Failed) != 0 {
		t.Fatalf("a running retry is not a failed task: %+v %+v", sum.Progress, sum.Failed)
	}
}

func TestExecutionSummary_DispatchErrorsDoNotUseUpAnAttempt(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeBug, domain.RequestSizeM)
	task := r.leaf(req, r.plan(req), "never ran", domain.TaskStatusOpen)
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	r.outcome(req, task, domain.OutcomeFailed, domain.CauseDispatchError, "TASK_EXECUTE_NO_CONNECTION", at)
	r.outcome(req, task, domain.OutcomeFailed, domain.CauseDispatchError, "TASK_EXECUTE_NO_CONNECTION", at.Add(time.Minute))

	sum, err := r.summaryUC(stubDecider{}).Execute(lcCtx(), req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Failed) != 1 || sum.Failed[0].Attempts != 0 || sum.Failed[0].LastCause != domain.FailureEnv {
		t.Fatalf("failed = %+v", sum.Failed)
	}
	if len(sum.NeedsYou) != 0 {
		t.Fatalf("a task that never ran is not out of attempts: %+v", sum.NeedsYou)
	}
}

func TestExecutionSummary_CurrentAndNextPhase(t *testing.T) {
	r := newExRig(t)
	req := r.continuousRequest(domain.RequestTypeChangeRequest, domain.RequestSizeM)
	plan := r.plan(req)
	p1, p2, p3 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2"), r.phase(req, plan, "p3")
	r.leaf(req, p1, "t1", domain.TaskStatusDone)
	r.leaf(req, p2, "t2", domain.TaskStatusInProgress)
	r.leaf(req, p3, "t3", domain.TaskStatusOpen)
	r.tasks.mu.Lock()
	r.tasks.setStatusLocked(p1.ID, domain.TaskStatusDone)
	r.tasks.mu.Unlock()
	r.starts.rows = append(r.starts.rows,
		domain.PhaseStart{RequestID: req.ID, PhaseTaskID: p1.ID}, domain.PhaseStart{RequestID: req.ID, PhaseTaskID: p2.ID})
	sum, err := r.summaryUC(stubDecider{}).Execute(lcCtx(), req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.CurrentPhase != p2.ID || sum.NextPhase != p3.ID || sum.Progress.PhasesDone != 1 || sum.Progress.PhasesTotal != 3 {
		t.Fatalf("current=%s next=%s progress=%+v", sum.CurrentPhase, sum.NextPhase, sum.Progress)
	}
}

func TestExecutionSummary_PendingApprovalsOnlyForThoseWhoMayDecide(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	ph := r.phase(req, plan, "the phase")
	gated := r.leaf(req, ph, "drop table", domain.TaskStatusOpen)
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPhase, SubjectID: ph.ID, Stage: "executing", Status: domain.ApprovalStatusPending})
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPhase, SubjectID: ph.ID, Stage: "drift_review", Status: domain.ApprovalStatusPending})
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPreDeploy, SubjectID: gated.ID, Stage: "executing", Status: domain.ApprovalStatusPending})
	r.gates.add(domain.Approval{RequestID: req.ID, SubjectType: domain.SubjectPlan, SubjectID: plan.ID, Stage: "awaiting_plan_approval", Status: domain.ApprovalStatusApproved})

	allowed, err := r.summaryUC(stubDecider{}).Execute(lcCtx(), req.ID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[domain.NeedsYouKind]int{}
	for _, n := range allowed.NeedsYou {
		kinds[n.Kind]++
	}
	if kinds[domain.NeedsApproval] != 1 || kinds[domain.NeedsDrift] != 1 || kinds[domain.NeedsPreDeploy] != 1 || len(allowed.NeedsYou) != 3 {
		t.Fatalf("needs you = %+v", allowed.NeedsYou)
	}
	for _, n := range allowed.NeedsYou {
		if n.Kind == domain.NeedsPreDeploy && n.Title != "drop table" || n.Kind == domain.NeedsApproval && n.Title != "the phase" {
			t.Errorf("titles come from the tree: %+v", n)
		}
	}
	denied, err := r.summaryUC(stubDecider{err: domain.ErrNotApprover}).Execute(lcCtx(), req.ID)
	if err != nil || len(denied.NeedsYou) != 0 {
		t.Fatalf("someone who may not decide sees nothing to do: %+v %v", denied.NeedsYou, err)
	}
	none, _ := r.summaryUC(nil).Execute(lcCtx(), req.ID)
	if len(none.NeedsYou) != 0 {
		t.Fatalf("without an authorizer nothing is listed: %+v", none.NeedsYou)
	}
}

func TestExecutionSummary_OpenClarificationOnlyForReporterAssigneesAndAdmins(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeM, domain.RequestStatusAwaitingInformation)
	due := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	uc := r.summaryUC(stubDecider{})
	uc.Clarifications = summaryClarifications{open: &domain.Clarification{
		ID: "c1", RequestID: req.ID, Source: domain.ClarificationSourceTaskBlocked, DueAt: due, CreatedAt: due.Add(-time.Hour),
		Assignees: []domain.Principal{{Kind: domain.PrincipalKindUser, ID: "assignee-1"}},
	}}
	cases := []struct {
		name string
		ctx  context.Context
		want int
	}{
		{"reporter", tenant.WithUserID(lcCtx(), req.ReporterID), 1},
		{"assignee", tenant.WithUserID(lcCtx(), "assignee-1"), 1},
		{"admin", tenant.WithRole(tenant.WithUserID(lcCtx(), "boss"), "admin"), 1},
		{"someone else", tenant.WithUserID(lcCtx(), "stranger"), 0},
		{"nobody", lcCtx(), 0},
	}
	for _, c := range cases {
		sum, err := uc.Execute(c.ctx, req.ID)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, item := range sum.NeedsYou {
			if item.Kind == domain.NeedsClarification {
				n++
			}
		}
		if n != c.want {
			t.Errorf("%s: %d clarification items, want %d", c.name, n, c.want)
		}
	}
}

func TestExecutionSummary_HandoverFollowsTheOpenSpecChange(t *testing.T) {
	cases := []struct {
		name    string
		status  domain.RequestStatus
		change  *domain.OpenSpecChange
		want    domain.HandoverState
		attempt int
	}{
		{"still executing", domain.RequestStatusExecuting, &domain.OpenSpecChange{Status: domain.ChangeStatusReady, SyncState: domain.SyncStateInSync}, domain.HandoverNone, 0},
		{"completed, archive not done", domain.RequestStatusCompleted, &domain.OpenSpecChange{Status: domain.ChangeStatusReady, SyncState: domain.SyncStatePending, SyncAttempts: 1}, domain.HandoverArchiving, 1},
		{"archived", domain.RequestStatusCompleted, &domain.OpenSpecChange{ChangeID: "add-audit", Status: domain.ChangeStatusArchived, SyncState: domain.SyncStateInSync}, domain.HandoverReady, 0},
		{"archive failed", domain.RequestStatusCompleted, &domain.OpenSpecChange{Status: domain.ChangeStatusReady, SyncState: domain.SyncStateFailed, SyncAttempts: 5}, domain.HandoverFailed, 5},
		{"no change row", domain.RequestStatusCompleted, nil, domain.HandoverNone, 0},
	}
	for _, c := range cases {
		r := newExRig(t)
		req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, c.status)
		uc := r.summaryUC(stubDecider{})
		uc.Changes = summaryChanges{change: c.change}
		sum, err := uc.Execute(lcCtx(), req.ID)
		if err != nil {
			t.Fatal(err)
		}
		if sum.Handover.State != c.want || sum.Handover.ArchiveAttempts != c.attempt {
			t.Errorf("%s: handover = %+v, want %s/%d", c.name, sum.Handover, c.want, c.attempt)
		}
	}
	// A native Request never has a handover, whatever the change table says.
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusCompleted)
	uc := r.summaryUC(stubDecider{})
	uc.Changes = summaryChanges{change: &domain.OpenSpecChange{Status: domain.ChangeStatusArchived}}
	if sum, _ := uc.Execute(lcCtx(), req.ID); sum.Handover.State != domain.HandoverNone || sum.Engine != domain.EngineNative {
		t.Fatalf("native: %+v engine=%s", sum.Handover, sum.Engine)
	}
}

func TestExecutionSummary_UnknownAndMalformedIDs(t *testing.T) {
	r := newExRig(t)
	uc := r.summaryUC(stubDecider{})
	if _, err := uc.Execute(lcCtx(), "not-a-uuid"); !errorHasCode(err, "REQUEST_NOT_FOUND") {
		t.Fatalf("got %v", err)
	}
	if _, err := uc.Execute(lcCtx(), "5f1c2a76-1111-4222-8333-944455556666"); !errorHasCode(err, "REQUEST_NOT_FOUND") {
		t.Fatalf("got %v", err)
	}
	if _, err := uc.Execute(context.Background(), "5f1c2a76-1111-4222-8333-944455556666"); err == nil {
		t.Fatal("a call without a tenant is refused")
	}
}
