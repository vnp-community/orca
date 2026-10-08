package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type exLocker struct{ r *exRig }

func (l exLocker) LockRequest(ctx context.Context, id string) (domain.Request, error) {
	return l.r.store.Get(ctx, id)
}

// handlerFor builds the same handler wireApproval builds for a subject backed by task-service artifacts.
func (r *exRig) handlerFor(artifacts SubjectArtifacts) *TransitionSubjectHandler {
	transition := newLcTransition(r.store)
	return &TransitionSubjectHandler{
		Artifacts: artifacts, Transition: transition, Requests: exLocker{r},
		Returner: NewReturnRequestToBacklog(r.store, transition, lcHistory{r.store}, &lcCanceller{}, r.guard, r.store, r.store),
	}
}

func phaseApproval(req domain.Request, st domain.SubjectType, subjectID string, status domain.RequestStatus) domain.Approval {
	return domain.Approval{ID: "ap-1", RequestID: req.ID, SubjectType: st, SubjectID: subjectID, Stage: string(status), Status: domain.ApprovalStatusPending, Comment: "not ready"}
}

func TestPhaseSubjectHandler_Validate_WrongPhase(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	r.phase(req, r.plan(req), "mine")
	other := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	foreign := r.phase(other, r.plan(other), "theirs")
	h := r.handlerFor(&PhaseArtifacts{Tasks: r.tasks})
	for _, hint := range []string{foreign.ID, "", "missing"} {
		ctx := WithRequestedSubjectID(lcCtx(), hint)
		if _, _, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPhase); !errorHasCode(err, "REQUEST_PHASE_NOT_IN_PLAN") {
			t.Errorf("hint %q: got %v", hint, err)
		}
	}
}

func TestPhaseSubjectHandler_Validate_RefusesOutsideExecuting(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusPlanning)
	ph := r.phase(req, r.plan(req), "p")
	h := r.handlerFor(&PhaseArtifacts{Tasks: r.tasks})
	ctx := WithRequestedSubjectID(lcCtx(), ph.ID)
	if _, _, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPhase); err == nil {
		t.Fatal("a phase gate only exists while executing")
	}
}

func TestPhaseSubjectHandler_DigestChangesOnNewTask(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	ph := r.phase(req, plan, "p")
	t1 := r.leaf(req, ph, "t1", domain.TaskStatusOpen)
	h := r.handlerFor(&PhaseArtifacts{Tasks: r.tasks})
	ctx := WithRequestedSubjectID(lcCtx(), ph.ID)
	digest := func() string {
		id, d, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPhase)
		if err != nil || id != ph.ID {
			t.Fatalf("validate: %q %v", id, err)
		}
		return d
	}
	base := digest()
	if digest() != base {
		t.Fatal("the digest must be stable")
	}
	t2 := r.leaf(req, ph, "t2", domain.TaskStatusOpen)
	withTwo := digest()
	if withTwo == base {
		t.Fatal("a new task must change the digest")
	}
	r.dependsOn(t2, t1)
	withEdge := digest()
	if withEdge == withTwo {
		t.Fatal("a new dependency must change the digest")
	}
	t1.Title = "renamed"
	r.tasks.tasks[t1.ID] = t1
	if digest() == withEdge {
		t.Fatal("a renamed task must change the digest")
	}
}

func TestPhaseSubjectHandler_OnRejected_ReturnsToBacklogPhase(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p")
	h := r.handlerFor(&PhaseArtifacts{Tasks: r.tasks})
	by := "approver-1"
	a := phaseApproval(req, domain.SubjectPhase, ph.ID, domain.RequestStatusExecuting)
	a.DecidedBy = &by
	if err := h.OnRejected(lcCtx(), lcCtx(), a); err != nil {
		t.Fatal(err)
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStagePhase || got.ReturnedCategory != domain.ReturnCategoryRejected || got.ReturnReason != "not ready" {
		t.Fatalf("a rejected phase returns the request from stage phase: %+v", got)
	}
}

func TestPhaseSubjectHandler_OnApproved_NoStateChange(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p")
	h := r.handlerFor(&PhaseArtifacts{Tasks: r.tasks})
	if err := h.OnApproved(lcCtx(), lcCtx(), phaseApproval(req, domain.SubjectPhase, ph.ID, domain.RequestStatusExecuting)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting || r.tasks.count("execute:") != 0 {
		t.Fatalf("approving a phase neither changes the request nor starts it: %+v %v", got, r.tasks.calls)
	}
}

func TestPreDeployHandler_Validate_Hotfix(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusAwaitingPlanApproval)
	fix := r.leaf(req, domain.TaskView{}, "fix", domain.TaskStatusOpen)
	other := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusAwaitingPlanApproval)
	foreign := r.leaf(other, domain.TaskView{}, "foreign", domain.TaskStatusOpen)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})

	ctx := WithRequestedSubjectID(lcCtx(), fix.ID)
	id, digest, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPreDeploy)
	if err != nil || id != fix.ID || digest == "" {
		t.Fatalf("got %q %q %v", id, digest, err)
	}
	ctx = WithRequestedSubjectID(lcCtx(), foreign.ID)
	if _, _, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPreDeploy); err == nil {
		t.Fatal("a task of another request is not this hotfix's subject")
	}
	// Without a hint the single fix task is found by itself.
	if id, _, err := h.ValidateForRequest(lcCtx(), lcCtx(), req, domain.SubjectPreDeploy); err != nil || id != fix.ID {
		t.Fatalf("got %q %v", id, err)
	}
	r.tasks.tasks[fix.ID] = withStatus(fix, domain.TaskStatusCancelled)
	if _, _, err := h.ValidateForRequest(lcCtx(), lcCtx(), req, domain.SubjectPreDeploy); err == nil {
		t.Fatal("a cancelled fix task cannot be approved")
	}
}

func TestPreDeployHandler_DigestChangesOnLabelEdit(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	step := r.leaf(req, r.plan(req), "drop", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	ctx := WithRequestedSubjectID(lcCtx(), step.ID)
	_, before, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPreDeploy)
	if err != nil {
		t.Fatal(err)
	}
	step.Labels = []string{domain.PolicyLabelGatePreDeploy, "risky"}
	r.tasks.tasks[step.ID] = step
	_, after, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPreDeploy)
	if err != nil || before == after {
		t.Fatalf("editing labels must change the digest: %v", err)
	}
	step.Labels = nil
	r.tasks.tasks[step.ID] = step
	if _, _, err := h.ValidateForRequest(ctx, ctx, req, domain.SubjectPreDeploy); !errorHasCode(err, "REQUEST_PRE_DEPLOY_REQUIRED") {
		t.Fatalf("a step without the gate label has nothing to approve: %v", err)
	}
}

func TestPreDeployHandler_Validate_Security_PlanCancelled(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeSecurity, domain.RequestSizeM, domain.RequestStatusAwaitingPlanApproval)
	plan := r.plan(req)
	r.leaf(req, plan, "patch", domain.TaskStatusOpen)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	id, digest, err := h.ValidateForRequest(lcCtx(), lcCtx(), req, domain.SubjectPreDeploy)
	if err != nil || id != plan.ID || digest == "" {
		t.Fatalf("got %q %q %v", id, digest, err)
	}
	r.tasks.tasks[plan.ID] = withStatus(plan, domain.TaskStatusCancelled)
	if _, _, err := h.ValidateForRequest(lcCtx(), lcCtx(), req, domain.SubjectPreDeploy); err == nil {
		t.Fatal("a cancelled plan cannot be approved")
	}
}

func TestPreDeployHandler_OnApproved_Hotfix_PlanApproved(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusAwaitingPlanApproval)
	fix := r.leaf(req, domain.TaskView{}, "fix", domain.TaskStatusOpen)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	by := "approver-1"
	a := phaseApproval(req, domain.SubjectPreDeploy, fix.ID, domain.RequestStatusAwaitingPlanApproval)
	a.DecidedBy = &by
	if err := h.OnApproved(lcCtx(), lcCtx(), a); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("approving the start gate starts execution, got %s", got.Status)
	}
}

func TestPreDeployHandler_OnRejected_Security_PlanRejected(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeSecurity, domain.RequestSizeM, domain.RequestStatusAwaitingPlanApproval)
	plan := r.plan(req)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	if err := h.OnRejected(lcCtx(), lcCtx(), phaseApproval(req, domain.SubjectPreDeploy, plan.ID, domain.RequestStatusAwaitingPlanApproval)); err != nil {
		t.Fatal(err)
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStagePlan || got.ReturnedCategory != domain.ReturnCategoryRejected {
		t.Fatalf("got %+v", got)
	}
}

func TestPreDeployHandler_OnApproved_Ops_NoStateChange(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	step := r.leaf(req, r.plan(req), "drop", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	if err := h.OnApproved(lcCtx(), lcCtx(), phaseApproval(req, domain.SubjectPreDeploy, step.ID, domain.RequestStatusExecuting)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("got %s", got.Status)
	}
}

func TestPreDeployHandler_OnRejected_Ops_BacklogTask(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	step := r.leaf(req, r.plan(req), "drop", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	if err := h.OnRejected(lcCtx(), lcCtx(), phaseApproval(req, domain.SubjectPreDeploy, step.ID, domain.RequestStatusExecuting)); err != nil {
		t.Fatal(err)
	}
	got := r.reload(req)
	if got.Status != domain.RequestStatusRequestBacklog || got.ReturnedFromStage != domain.ReturnStageTask || got.ReturnedCategory != domain.ReturnCategoryRejected {
		t.Fatalf("got %+v", got)
	}
}

func TestPreDeployArtifacts_TaskServiceDown(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusAwaitingPlanApproval)
	r.tasks.listErr = domain.ErrTaskServiceDown
	h := r.handlerFor(&PreDeployArtifacts{Tasks: r.tasks})
	if _, _, err := h.ValidateForRequest(lcCtx(), lcCtx(), req, domain.SubjectPreDeploy); !errorHasCode(err, "REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE") {
		t.Fatalf("an outage must not look like a missing subject: %v", err)
	}
}
