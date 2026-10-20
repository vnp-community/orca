package usecase

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// revisingKit is a Request whose Plan waits for approval, with the revision worker attached to GeneratePlan.
func revisingKit(t *testing.T) *workerKit {
	t.Helper()
	k := newWorkerKit(t, domain.RequestTypeTask)
	r := k.w.requests[k.req.ID]
	r.Status, r.PlanTaskID = domain.RequestStatusAwaitingPlanApproval, "11111111-1111-4111-8111-111111111111"
	k.w.requests[k.req.ID] = r
	k.req = r
	k.generate.tx = planAutoTx{w: k.w, proc: newMemProcessedEvents(), jobs: k.jobs}
	k.generate.WithRevisionWorker(k.jobs, NewPlanActorAuthorizer(allowAll{}, nil, true))
	return k
}

func (k *workerKit) reviseAs(t *testing.T, user, feedback string) GeneratePlanResult {
	t.Helper()
	res, err := k.generate.Execute(tenant.WithUserID(planAutoTenantCtx(), user), GeneratePlanInput{RequestID: k.req.ID, Feedback: feedback})
	if err != nil || !res.Created {
		t.Fatalf("revise: %+v %v", res, err)
	}
	return res
}

func TestPlanAutoWorker_CommitsARevisedPlanForAnApproverWhoMayNotCommit(t *testing.T) {
	k := revisingKit(t)
	approver := uuid.NewString()
	res := k.reviseAs(t, approver, "split the migration")
	job := k.jobs.get(k.req.ID, domain.RevisionAnchor(res.RunID))
	if job.Origin != domain.PlanAutoOriginRevision || job.State != domain.PlanAutoRunning {
		t.Fatalf("job = %+v", job)
	}

	k.pass(0) // the run is still going: the job keeps watching
	if got := k.jobs.get(k.req.ID, domain.RevisionAnchor(res.RunID)); got.State != domain.PlanAutoRunning {
		t.Fatalf("job = %+v", got)
	}
	k.succeedRun(t, res.RunID)
	k.pass(k.policy.PollInterval)
	k.pass(0)

	got := k.jobs.get(k.req.ID, domain.RevisionAnchor(res.RunID))
	if got.State != domain.PlanAutoDone {
		t.Fatalf("job = %+v", got)
	}
	cur := k.w.requests[k.req.ID]
	if cur.Status != domain.RequestStatusAwaitingPlanApproval || cur.PlanTaskID == "11111111-1111-4111-8111-111111111111" || cur.PlanTaskID == "" {
		t.Fatalf("the revised Plan must be committed and wait for approval again: %s plan=%q", cur.Status, cur.PlanTaskID)
	}
	if len(k.opener.opened) != 1 {
		t.Fatalf("approvals opened = %d", len(k.opener.opened))
	}
	if k.w.runs[res.RunID].ActorID != approver {
		t.Fatal("the run stays the approver's even though the reporter's identity commits it")
	}
}

func TestPlanAutoWorker_ARevisionThatFailsIsNotRepeatedByTheWorker(t *testing.T) {
	k := revisingKit(t)
	res := k.reviseAs(t, k.req.ReporterID, "split the migration")
	k.failRun(res.RunID, domain.PlanAutoCodeCompleteTimeout) // transient for an automatic Plan, but a revision has one attempt
	k.pass(0)
	got := k.jobs.get(k.req.ID, domain.RevisionAnchor(res.RunID))
	if got.State != domain.PlanAutoGaveUp || got.ReasonCode != domain.PlanAutoCodeCompleteTimeout {
		t.Fatalf("job = %+v", got)
	}
	if k.start.calls != 0 || len(k.planRuns()) != 1 {
		t.Fatalf("the worker must not start another run: %d call(s), %d run(s)", k.start.calls, len(k.planRuns()))
	}
	if k.w.requests[k.req.ID].Status != domain.RequestStatusPlanning {
		t.Fatal("the Request stays in planning for the person to retry")
	}
}
