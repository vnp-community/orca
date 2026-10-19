package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// seedFinishedPhaseOne gives a change_request whose first Phase is done, so the next Phase's gate is due.
func seedFinishedPhaseOne(r *exRig) (domain.Request, domain.TaskView) {
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	r.leaf(req, p1, "t1", domain.TaskStatusDone)
	r.tasks.setStatusLocked(p1.ID, domain.TaskStatusDone)
	r.leaf(req, p2, "t2", domain.TaskStatusOpen)
	r.dependsOn(p2, p1)
	r.approved(req, domain.SubjectPlan, plan.ID)
	r.approved(req, domain.SubjectPhase, p1.ID)
	r.starts.rows = append(r.starts.rows, domain.PhaseStart{PhaseTaskID: p1.ID, RequestID: req.ID, StartedBy: "approver-1"})
	return req, p2
}

// Regression for the Phase gate that never opened: the consumers and the reconcile loop carry a tenant only, while
// task-service shows the Plan tree to its owner. Opening must run as the execution actor.
func TestEvaluateExecution_OpensThePhaseGateFromABackgroundContext(t *testing.T) {
	r := newExRig(t)
	r.tasks.treeOwner = "approver-1" // exActors answers approver-1, who owns the tree
	r.opener.validate, r.opener.requests = &PhaseArtifacts{Tasks: r.tasks}, r.store
	req, next := seedFinishedPhaseOne(r)

	if err := r.evaluate.Run(lcCtx(), req); err != nil {
		t.Fatalf("a background context must still open the next Phase's gate: %v", err)
	}
	if len(r.opener.calls) != 1 || r.opener.calls[0].SubjectType != domain.SubjectPhase || r.opener.calls[0].SubjectID != next.ID {
		t.Fatalf("one phase approval for the next phase: %+v", r.opener.calls)
	}
	if r.opener.users[0] != "approver-1" {
		t.Fatalf("the gate must open as the execution actor, got %q", r.opener.users[0])
	}
}

func TestEvaluateExecution_PhaseGateReportsAnActorWhoCannotSeeThePhase(t *testing.T) {
	r := newExRig(t)
	r.tasks.treeOwner = "someone-else"
	r.opener.validate, r.opener.requests = &PhaseArtifacts{Tasks: r.tasks}, r.store
	req, _ := seedFinishedPhaseOne(r)

	err := r.evaluate.Run(lcCtx(), req)
	if !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("a Phase the actor cannot read is a permission problem, not a missing Phase: %v", err)
	}
}

func TestPhaseArtifacts_Describe_NotInPlanStaysNotInPlan(t *testing.T) {
	r := newExRig(t)
	r.tasks.treeOwner = "approver-1"
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	other := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	foreign := r.phase(other, r.plan(other), "theirs")
	a := &PhaseArtifacts{Tasks: r.tasks}
	ctx := tenant.WithUserID(lcCtx(), "approver-1")
	for _, id := range []string{foreign.ID, "missing"} {
		if _, _, err := a.Describe(ctx, req, domain.SubjectPhase, id); !errorHasCode(err, "REQUEST_PHASE_NOT_IN_PLAN") {
			t.Errorf("phase %q: got %v", id, err)
		}
	}
}

func TestAdvanceExecution_PreDeployGateOpensAsTheActor(t *testing.T) {
	r := newExRig(t)
	r.tasks.treeOwner = "approver-1"
	r.opener.validate, r.opener.requests = &PreDeployArtifacts{Tasks: r.tasks}, r.store
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	gated := r.leaf(req, r.plan(req), "drop table", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)

	// Run from the evaluation loop's tenant-only context, as the reconcile loop does.
	if err := r.evaluate.Run(lcCtx(), req); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(r.opener.calls) != 1 || r.opener.calls[0].SubjectID != gated.ID || r.opener.users[0] != "approver-1" {
		t.Fatalf("pre_deploy gate must open as the actor: calls=%+v users=%v", r.opener.calls, r.opener.users)
	}
}

func TestStartPhase_ActorWhoCannotSeeThePhaseIsRefusedNotWavedThrough(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.leaf(req, p2, "t2", domain.TaskStatusBlocked)
	r.dependsOn(p2, p1)
	r.approved(req, domain.SubjectPhase, p2.ID)
	r.tasks.treeOwner = "someone-else"

	if _, err := startPhase(r, req, p2.ID); !errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
		t.Fatalf("with no edges visible the predecessor test would pass by accident: %v", err)
	}
	if r.tasks.count("execute:") != 0 {
		t.Fatalf("nothing may run: %v", r.tasks.calls)
	}
}
