package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ownedPlan is a Plan committed by "committer", the only user task-service lets write grants on it.
func ownedPlan(r *exRig, req domain.Request) domain.TaskView {
	plan := r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan", OwnerID: "committer"})
	r.tasks.executors = map[string]bool{"committer": true}
	return plan
}

func newAccess(r *exRig) *ExecutionAccess { return &ExecutionAccess{Tasks: r.tasks} }

func TestExecutionAccess_ApproverWhoDidNotCommitThePlanIsGrantedExecute(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := ownedPlan(r, req)

	if err := newAccess(r).Ensure(lcCtx(), req, "approver-1"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(r.tasks.grants) != 1 || r.tasks.grants[0] != "committer>approver-1@"+plan.ID {
		t.Fatalf("the grant is written as the Plan's owner, on the Plan root: %v", r.tasks.grants)
	}
	if ok, _ := r.tasks.CanExecute(lcCtx(), plan.ID, "approver-1"); !ok {
		t.Fatal("the approver can now execute")
	}
}

func TestExecutionAccess_NoGrantWhenTheActorAlreadyCanExecute(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ownedPlan(r, req)

	if err := newAccess(r).Ensure(lcCtx(), req, "committer"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(r.tasks.grants) != 0 {
		t.Fatalf("the owner needs no grant: %v", r.tasks.grants)
	}
}

func TestExecutionAccess_RepeatedCallsAskTaskServiceOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ownedPlan(r, req)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	a := &ExecutionAccess{Tasks: r.tasks, Now: func() time.Time { return now }}

	for range 3 {
		if err := a.Ensure(lcCtx(), req, "approver-1"); err != nil {
			t.Fatal(err)
		}
	}
	if n := r.tasks.count("canexecute:"); n != 1 {
		t.Fatalf("a confirmed grant is trusted for a while: %d permission checks", n)
	}
	now = now.Add(executionAccessTTL + time.Second)
	if err := a.Ensure(lcCtx(), req, "approver-1"); err != nil {
		t.Fatal(err)
	}
	if n := r.tasks.count("canexecute:"); n != 2 || len(r.tasks.grants) != 1 {
		t.Fatalf("after the TTL it asks again and finds the grant in place: checks=%d grants=%v", n, r.tasks.grants)
	}
}

func TestExecutionAccess_PlanWithoutAnOwnerIsRefused(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	r.plan(req)
	r.tasks.executors = map[string]bool{}

	if err := newAccess(r).Ensure(lcCtx(), req, "approver-1"); !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("nobody can write the grant, so say forbidden instead of guessing an owner: %v", err)
	}
	if len(r.tasks.grants) != 0 {
		t.Fatalf("no grant: %v", r.tasks.grants)
	}
}

func TestExecutionAccess_GrantRefusedByTaskServiceIsForbidden(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ownedPlan(r, req)
	if err := (&ExecutionAccess{Tasks: &denyingGrants{r.tasks}}).Ensure(lcCtx(), req, "approver-1"); !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("a refused grant is a permission error: %v", err)
	}
}

type denyingGrants struct{ *exTasks }

func (denyingGrants) GrantExecute(context.Context, string, string, string) error {
	return domain.ErrTaskForbidden
}

func TestExecutionAccess_TaskServiceOutageIsNotAPermissionError(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ownedPlan(r, req)
	r.tasks.listErr = domain.ErrTaskServiceDown

	err := newAccess(r).Ensure(lcCtx(), req, "approver-1")
	if !errors.Is(err, domain.ErrTaskServiceDown) || errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("an outage is retried, not reported as forbidden: %v", err)
	}
}

func TestExecutionAccess_HotfixFixTaskWithoutAPlanIsTheRoot(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusExecuting)
	fix := r.tasks.add(domain.TaskView{Type: "bug", Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "fix", OwnerID: "committer"})
	r.tasks.executors = map[string]bool{"committer": true}

	if err := newAccess(r).Ensure(lcCtx(), req, "approver-1"); err != nil {
		t.Fatal(err)
	}
	if len(r.tasks.grants) != 1 || r.tasks.grants[0] != "committer>approver-1@"+fix.ID {
		t.Fatalf("the parentless fix task is granted: %v", r.tasks.grants)
	}
}

func TestExecutionAccess_CancelledPlanIsIgnoredAndNoTreeNeedsNothing(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	if err := newAccess(r).Ensure(lcCtx(), req, "approver-1"); err != nil {
		t.Fatalf("no tree yet: %v", err)
	}
	old := ownedPlan(r, req)
	r.tasks.setStatusLocked(old.ID, domain.TaskStatusCancelled)
	r.tasks.grants = nil
	if err := newAccess(r).Ensure(lcCtx(), req, "approver-1"); err != nil || len(r.tasks.grants) != 0 {
		t.Fatalf("a superseded plan is not the one to run: %v %v", err, r.tasks.grants)
	}
}

func TestApprovalExecutionActors_ActorIsMadeAbleToExecute(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := ownedPlan(r, req)
	r.approved(req, domain.SubjectPlan, plan.ID) // decided by approver-1, not by the committer
	actors := &ApprovalExecutionActors{Approvals: r.gates, PhaseStarts: r.starts, Access: newAccess(r)}

	got, err := actors.ActorFor(lcCtx(), req, "")
	if err != nil || got != "approver-1" {
		t.Fatalf("the Plan approver stays the actor: %q %v", got, err)
	}
	if ok, _ := r.tasks.CanExecute(lcCtx(), plan.ID, "approver-1"); !ok {
		t.Fatal("and may run the tree even though someone else committed the Plan")
	}
}

func TestApprovalExecutionActors_RefusedActorReportsForbiddenInsteadOfDispatchingBlind(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req) // no owner on record: nobody can grant
	r.tasks.executors = map[string]bool{}
	r.approved(req, domain.SubjectPlan, plan.ID)
	actors := &ApprovalExecutionActors{Approvals: r.gates, PhaseStarts: r.starts, Access: newAccess(r)}

	if _, err := actors.ActorFor(lcCtx(), req, ""); !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("got %v", err)
	}
}

func TestStartPhase_StarterWhoDidNotCommitThePlanIsGrantedBeforeTheClaim(t *testing.T) {
	r := newExRig(t)
	r.startPh.Access = newAccess(r)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := ownedPlan(r, req)
	p1 := r.phase(req, plan, "p1")
	r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.approved(req, domain.SubjectPhase, p1.ID)

	res, err := startPhase(r, req, p1.ID)
	if err != nil || len(res.Dispatched) != 1 {
		t.Fatalf("starter-1 did not commit the Plan but may start the Phase: %+v %v", res, err)
	}
	if len(r.tasks.grants) != 1 || r.tasks.grants[0] != "committer>starter-1@"+plan.ID {
		t.Fatalf("grants: %v", r.tasks.grants)
	}
}

func TestStartPhase_UngrantableStarterLeavesNoClaimBehind(t *testing.T) {
	r := newExRig(t)
	r.startPh.Access = newAccess(r)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	r.tasks.executors = map[string]bool{}
	p1 := r.phase(req, plan, "p1")
	r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.approved(req, domain.SubjectPhase, p1.ID)

	if _, err := startPhase(r, req, p1.ID); !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("got %v", err)
	}
	if rows, _ := r.starts.ListByRequest(lcCtx(), req.ID); len(rows) != 0 {
		t.Fatalf("a start nobody can run must not become the Phase's actor: %+v", rows)
	}
}
