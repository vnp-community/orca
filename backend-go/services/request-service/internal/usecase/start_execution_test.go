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
