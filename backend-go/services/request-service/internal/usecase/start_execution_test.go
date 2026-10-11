package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestStatusConsumer_ExecutingStartsPlanWithoutPhase(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeTask, domain.RequestTypeDocs, domain.RequestTypeBug, domain.RequestTypeSecurity, domain.RequestTypePerformance} {
		r := newExRig(t)
		req := r.request(typ, domain.RequestSizeS, domain.RequestStatusExecuting)
		task := r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
		started, err := r.startExec.Execute(lcCtx(), req.ID)
		if err != nil || !started || r.tasks.get(task.ID).Status != domain.TaskStatusInProgress {
			t.Fatalf("%s: started=%v err=%v calls=%v", typ, started, err, r.tasks.calls)
		}
	}
}

func TestStatusConsumer_ChangeRequestNotAutoStarted(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p1")
	r.leaf(req, ph, "t", domain.TaskStatusOpen)
	started, err := r.startExec.Execute(lcCtx(), req.ID)
	if err != nil || started || r.tasks.count("execute:") != 0 {
		t.Fatalf("a phased request waits for StartPhase: %v %v %v", started, err, r.tasks.calls)
	}
	bug := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	r.leaf(bug, r.phase(bug, r.plan(bug), "p"), "t", domain.TaskStatusOpen)
	if started, _ := r.startExec.Execute(lcCtx(), bug.ID); started {
		t.Fatal("a size L bug is phased and must wait too")
	}
}

func TestStatusConsumer_DuplicateDelivery_StartsOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	for i := 0; i < 3; i++ {
		if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
			t.Fatal(err)
		}
	}
	if r.tasks.count("execute:") != 1 {
		t.Fatalf("Execute must run once: %v", r.tasks.calls)
	}
}

func TestStatusConsumer_HotfixStartsSingleTask(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusExecuting)
	fix := r.leaf(req, domain.TaskView{}, "fix", domain.TaskStatusOpen)
	started, err := r.startExec.Execute(lcCtx(), req.ID)
	if err != nil || !started || r.tasks.get(fix.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("got %v %v %v", started, err, r.tasks.calls)
	}
}

func TestStartExecution_IgnoresRequestsNotExecuting(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusPlanning)
	r.leaf(req, r.plan(req), "t", domain.TaskStatusOpen)
	if started, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || started || len(r.tasks.calls) != 0 {
		t.Fatalf("got %v %v %v", started, err, r.tasks.calls)
	}
}

func TestResumeAfterGate(t *testing.T) {
	r := newExRig(t)
	resume := &ResumeAfterGate{Requests: r.store, Evaluate: r.evaluate}
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	gated := r.leaf(req, r.plan(req), "drop", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)

	if err := resume.Execute(lcCtx(), req.ID, domain.SubjectPhase); err != nil || len(r.tasks.calls) != 0 {
		t.Fatalf("a phase approval resumes nothing: %v %v", err, r.tasks.calls)
	}
	r.approved(req, domain.SubjectPreDeploy, gated.ID)
	if err := resume.Execute(lcCtx(), req.ID, domain.SubjectPreDeploy); err != nil {
		t.Fatal(err)
	}
	if r.tasks.get(gated.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("an approved pre_deploy releases the waiting task: %v", r.tasks.calls)
	}
	if err := resume.Execute(lcCtx(), "no-such-request", domain.SubjectPreDeploy); err != nil {
		t.Fatalf("an unknown request is ignored: %v", err)
	}
}

func pendingPhaseApprovals(r *exRig) []string {
	var ids []string
	for _, a := range r.gates.approvals {
		if a.SubjectType == domain.SubjectPhase && a.Status == domain.ApprovalStatusPending {
			ids = append(ids, a.SubjectID)
		}
	}
	return ids
}

// A gated flow (change_request) needs the first Phase's Approval open as soon as the Request is executing, or nobody can start it.
func TestStatusConsumer_GatedFlowOpensTheFirstPhaseApprovalOnce(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	p1, p2 := r.phase(req, plan, "p1"), r.phase(req, plan, "p2")
	r.leaf(req, p1, "t1", domain.TaskStatusOpen)
	r.leaf(req, p2, "t2", domain.TaskStatusOpen)

	for i := 0; i < 3; i++ { // the status event may be delivered more than once
		if started, err := r.startExec.Execute(lcCtx(), req.ID); err != nil || started {
			t.Fatalf("a phased request is not auto-started: %v %v", started, err)
		}
	}
	if got := pendingPhaseApprovals(r); len(got) != 1 || got[0] != p1.ID {
		t.Fatalf("pending phase approvals = %v, want only the first phase %s", got, p1.ID)
	}
	if r.tasks.count("execute:") != 0 {
		t.Fatalf("nothing runs before a person starts the phase: %v", r.tasks.calls)
	}
}

// task-service does not list Phases in dependency order. The first gate must go to the Phase that can start; a gate for the
// Phase that waits on another one cannot be started (REQUEST_PHASE_PREDECESSOR_NOT_DONE) and leaves the Request stuck.
func TestStatusConsumer_FirstPhaseApprovalSkipsAPhaseThatWaitsForAnother(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	dependent := r.phase(req, plan, "tests, after the module")
	module := r.phase(req, plan, "the module")
	r.dependsOn(dependent, module)
	r.leaf(req, dependent, "t-tests", domain.TaskStatusBlocked)
	r.leaf(req, module, "t-module", domain.TaskStatusOpen)

	if _, err := r.startExec.Execute(lcCtx(), req.ID); err != nil {
		t.Fatal(err)
	}
	if got := pendingPhaseApprovals(r); len(got) != 1 || got[0] != module.ID {
		t.Fatalf("pending phase approvals = %v, want only %s (the phase nothing waits on first), not %s", got, module.ID, dependent.ID)
	}
}

func TestStatusConsumer_FlowsWithoutPhaseGateOpenNoPhaseApproval(t *testing.T) {
	r := newExRig(t)
	bug := r.request(domain.RequestTypeBug, domain.RequestSizeL, domain.RequestStatusExecuting)
	r.leaf(bug, r.phase(bug, r.plan(bug), "p"), "t", domain.TaskStatusOpen)
	if _, err := r.startExec.Execute(lcCtx(), bug.ID); err != nil {
		t.Fatal(err)
	}
	if got := pendingPhaseApprovals(r); len(got) != 0 {
		t.Fatalf("the plan approval already covers a size L bug's phases, got %v", got)
	}
}
