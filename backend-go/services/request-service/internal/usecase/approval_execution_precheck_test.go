package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type denyingAuthorizer struct{ err error }

func (a denyingAuthorizer) CanDecide(context.Context, domain.Request, domain.Approval) error {
	return a.err
}

func precheckFor(r *exRig, settings *domain.ProjectEngineSettings, authz ApprovalAuthorizer) *ExecutionStartPrecheck {
	return &ExecutionStartPrecheck{
		Requests: r.store, Access: newAccess(r), Authorizer: authz,
		Modes: &ExecutionModeResolver{Settings: staticEngineSettings{settings}},
	}
}

func planApproval(req domain.Request, st domain.SubjectType) domain.Approval {
	return domain.Approval{RequestID: req.ID, SubjectType: st, Stage: string(domain.RequestStatusAwaitingPlanApproval), Status: domain.ApprovalStatusPending}
}

func awaitingPlan(r *exRig, mode domain.ExecutionMode) domain.Request {
	return r.store.seed(func(q *domain.Request) {
		q.Type, q.Size, q.Status, q.ExecutionMode = domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusAwaitingPlanApproval, mode
	})
}

func TestStartPrecheck_ContinuousRefusesAnApproverWhoCannotRunThePlan(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, domain.ExecutionContinuous)
	// Nobody owns the tree, so no grant can ever be written for the approver.
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}

	err := precheckFor(r, nil, nil).Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", "")
	if !errorHasCode(err, "REQUEST_EXECUTION_ACTOR_NOT_ALLOWED") {
		t.Fatalf("got %v, want REQUEST_EXECUTION_ACTOR_NOT_ALLOWED", err)
	}
}

func TestStartPrecheck_AnApproverWhoCanBeGrantedAccessPassesWithoutAGrant(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, domain.ExecutionContinuous)
	ownedPlan(r, req)
	if err := precheckFor(r, nil, nil).Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", ""); err != nil {
		t.Fatalf("an owner exists who can write the grant: %v", err)
	}
	if len(r.tasks.grants) != 0 {
		t.Fatalf("the precheck must not write a grant before the decision: %v", r.tasks.grants)
	}
}

func TestStartPrecheck_OnlyContinuousRunsAreChecked(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, domain.ExecutionStepByStep)
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	if err := precheckFor(r, nil, nil).Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", ""); err != nil {
		t.Fatalf("step_by_step keeps its behaviour: %v", err)
	}
}

func TestStartPrecheck_UnpinnedRequestFollowsTheProjectSetting(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, "")
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	pre := precheckFor(r, &domain.ProjectEngineSettings{ExecutionMode: domain.ExecutionContinuous}, nil)
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", ""); !errorHasCode(err, "REQUEST_EXECUTION_ACTOR_NOT_ALLOWED") {
		t.Fatalf("the mode the Request will be pinned with is the one checked: %v", err)
	}
}

func TestStartPrecheck_OtherSubjectsAndStagesAreNotChecked(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, domain.ExecutionContinuous)
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	pre := precheckFor(r, nil, nil)
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectSolution), "approver-1", ""); err != nil {
		t.Fatalf("a solution approval does not start a run: %v", err)
	}
	wrongStage := planApproval(req, domain.SubjectPlan)
	wrongStage.Stage = string(domain.RequestStatusExecuting)
	if err := pre.Check(lcCtx(), wrongStage, "approver-1", ""); err != nil {
		t.Fatalf("only the approval that occupies awaiting_plan_approval starts a run: %v", err)
	}
}

func TestStartPrecheck_SomeoneWhoMayNotDecideGetsTheNormalRefusalNotATaskAccessHint(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, domain.ExecutionContinuous)
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	pre := precheckFor(r, nil, denyingAuthorizer{err: domain.ErrNotApprover})
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "stranger", ""); err != nil {
		t.Fatalf("the decision itself reports who may approve, so the precheck stays silent: %v", err)
	}
}

func TestStartPrecheck_ATaskServiceErrorIsNotTurnedIntoARefusal(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, domain.ExecutionContinuous)
	boom := errors.New("task-service unavailable")
	pre := precheckFor(r, nil, nil)
	pre.Requests = failingRequests{err: boom}
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", ""); !errors.Is(err, boom) {
		t.Fatalf("got %v, want the original error", err)
	}
}

func TestStartPrecheck_NilIsNoop(t *testing.T) {
	var pre *ExecutionStartPrecheck
	if err := pre.Check(lcCtx(), domain.Approval{}, "x", ""); err != nil {
		t.Fatal(err)
	}
}

type failingRequests struct{ err error }

func (f failingRequests) Get(context.Context, string) (domain.Request, error) {
	return domain.Request{}, f.err
}

// Choosing to run everything makes the approver the execution actor even where the project is step by step.
func TestStartPrecheck_RequestedContinuousInAStepByStepProjectChecksTheApprover(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, "")
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	pre := precheckFor(r, &domain.ProjectEngineSettings{ExecutionMode: domain.ExecutionStepByStep}, nil)

	err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", domain.ExecutionContinuous)
	if !errorHasCode(err, "REQUEST_EXECUTION_ACTOR_NOT_ALLOWED") {
		t.Fatalf("got %v, want REQUEST_EXECUTION_ACTOR_NOT_ALLOWED", err)
	}
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", ""); err != nil {
		t.Fatalf("no choice in a step by step project keeps the old behaviour (nothing checked): %v", err)
	}
}

func TestStartPrecheck_RequestedContinuousPassesForAnApproverWhoMayExecute(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, "")
	ownedPlan(r, req)
	pre := precheckFor(r, &domain.ProjectEngineSettings{ExecutionMode: domain.ExecutionStepByStep}, nil)
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "approver-1", domain.ExecutionContinuous); err != nil {
		t.Fatalf("an owner exists who can write the grant: %v", err)
	}
}

func TestStartPrecheck_RequestedModeIsIgnoredForApprovalsThatDoNotStartARun(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, "")
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	pre := precheckFor(r, nil, nil)
	for _, st := range []domain.SubjectType{domain.SubjectPhase, domain.SubjectSolution, domain.SubjectRequestType} {
		if err := pre.Check(lcCtx(), planApproval(req, st), "approver-1", domain.ExecutionContinuous); err != nil {
			t.Fatalf("%s does not start a run, the usecase refuses the mode before this runs: %v", st, err)
		}
	}
	wrongStage := planApproval(req, domain.SubjectPlan)
	wrongStage.Stage = string(domain.RequestStatusExecuting)
	if err := pre.Check(lcCtx(), wrongStage, "approver-1", domain.ExecutionContinuous); err != nil {
		t.Fatalf("wrong stage: %v", err)
	}
}

func TestStartPrecheck_RequestedContinuousFromSomeoneWhoMayNotDecideLeavesTheRefusalToTheDecision(t *testing.T) {
	r := newExRig(t)
	req := awaitingPlan(r, "")
	r.tasks.add(domain.TaskView{Type: domain.TaskTypePlan, Status: domain.TaskStatusOpen, RequestID: req.ID, Title: "plan"})
	r.tasks.executors = map[string]bool{"committer": true}
	pre := precheckFor(r, nil, denyingAuthorizer{err: domain.ErrNotApprover})
	if err := pre.Check(lcCtx(), planApproval(req, domain.SubjectPlan), "stranger", domain.ExecutionContinuous); err != nil {
		t.Fatalf("got %v, want nil so CanDecide reports who may approve", err)
	}
}
