package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func commitKit(t *testing.T, typ domain.RequestType, size domain.RequestSize) (*planKit, domain.Request) {
	t.Helper()
	k := newPlanKit(nil)
	req := k.w.seedRequest(typ, domain.RequestStatusPlanning)
	r := k.w.requests[req.ID]
	r.Size = size
	k.w.requests[req.ID] = r
	return k, r
}

func proposalOf(t *testing.T, phased bool, labels ...string) domain.PlanProposal {
	t.Helper()
	p, err := ParsePlanOutput(planJSON(phased, labels...))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCommitPlan_WritesThePlanThenMovesToApproval(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM)
	res, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, true)})
	if err != nil || res.PlanTaskID == "" || len(res.PhaseTaskIDs) != 1 || len(res.TaskIDs) != 1 {
		t.Fatalf("commit: %+v %v", res, err)
	}
	got := k.w.requests[req.ID]
	if got.Status != domain.RequestStatusAwaitingPlanApproval || got.PlanTaskID != res.PlanTaskID {
		t.Fatalf("request = %s plan=%s, want awaiting_plan_approval with plan %s", got.Status, got.PlanTaskID, res.PlanTaskID)
	}
	if len(k.opener.opened) != 1 || k.opener.opened[0].SubjectType != domain.SubjectPlan || k.opener.opened[0].SubjectID != res.PlanTaskID {
		t.Fatalf("approval opened = %+v", k.opener.opened)
	}
	if !contains(k.w.outboxSubjects(), domain.SubjectPlanGenerated) {
		t.Fatalf("plan.generated not emitted: %v", k.w.outboxSubjects())
	}
	call := k.writer.calls[0]
	if call.SupersedesPlanID != "" || call.CreatorID == "" || call.AIPlanJSON == "" || call.Shape != domain.ShapePlanPhases {
		t.Fatalf("task-service call = %+v", call)
	}
}

func TestCommitPlan_OpensTheGateOfTheType(t *testing.T) {
	cases := []struct {
		typ  domain.RequestType
		size domain.RequestSize
		want domain.SubjectType
		with bool
	}{
		{domain.RequestTypeTask, domain.RequestSizeS, domain.SubjectTaskList, false},
		{domain.RequestTypeDocs, domain.RequestSizeS, domain.SubjectTaskList, false},
		{domain.RequestTypeSecurity, domain.RequestSizeM, domain.SubjectPreDeploy, false},
		{domain.RequestTypeBug, domain.RequestSizeL, domain.SubjectPlan, true},
	}
	for _, c := range cases {
		k, req := commitKit(t, c.typ, c.size)
		labels := []string{}
		if c.typ == domain.RequestTypeBug || c.typ == domain.RequestTypeSecurity {
			labels = append(labels, domain.PlanLabelTestRegression)
		}
		if _, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, c.with, labels...)}); err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		if len(k.opener.opened) != 1 || k.opener.opened[0].SubjectType != c.want {
			t.Errorf("%s: approval = %+v, want %s", c.typ, k.opener.opened, c.want)
		}
	}
}

func TestCommitPlan_RefusesWhatItCannotTrust(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM)
	_, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)}) // flat plan, phases required
	requireCode(t, err, "REQUEST_PLAN_MIXED_CHILDREN")
	if k.writer.callCount() != 0 || k.w.requests[req.ID].Status != domain.RequestStatusPlanning {
		t.Fatal("an invalid proposal must change nothing")
	}

	other := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusAnalyzing)
	_, err = k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: other.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_STATE_STALE")

	bug, _ := commitKit(t, domain.RequestTypeBug, domain.RequestSizeM)
	bugReq := bug.w.seedRequest(domain.RequestTypeBug, domain.RequestStatusPlanning)
	_, err = bug.commit.Execute(bug.w.ctx(), CommitPlanInput{RequestID: bugReq.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_PLAN_REGRESSION_TEST_MISSING")
}

func TestCommitPlan_TaskServiceFailureChangesNothing(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	k.writer.err = domain.ErrPlanTaskServiceUnavailable(errBoom)
	_, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_PLAN_TASK_SERVICE_UNAVAILABLE")
	if got := k.w.requests[req.ID]; got.Status != domain.RequestStatusPlanning || got.PlanTaskID != "" {
		t.Fatalf("request moved: %s plan=%q", got.Status, got.PlanTaskID)
	}
}

// If the approval cannot be opened the whole request-service transaction rolls back; calling again finishes the job
// and task-service reports the Plan it already holds instead of writing a second one.
func TestCommitPlan_ApprovalFailureRollsBackAndTheRetryFinishes(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	k.opener.err = errBoom
	if _, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)}); err == nil {
		t.Fatal("want the approval failure")
	}
	if got := k.w.requests[req.ID]; got.Status != domain.RequestStatusPlanning || got.PlanTaskID != "" {
		t.Fatalf("a failed commit must roll back: %s plan=%q", got.Status, got.PlanTaskID)
	}
	k.opener.err = nil
	res, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil || !res.AlreadyExists {
		t.Fatalf("retry: %+v %v", res, err)
	}
	if got := k.w.requests[req.ID]; got.Status != domain.RequestStatusAwaitingPlanApproval || got.PlanTaskID != res.PlanTaskID {
		t.Fatalf("retry did not finish the commit: %s", got.Status)
	}
	if len(k.writer.trees) != 1 {
		t.Fatal("task-service must hold exactly one Plan")
	}
}

func TestCommitPlan_ReplayOfASuccessfulCommitIsAnswered(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	first, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil {
		t.Fatal(err)
	}
	again, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil || again.PlanTaskID != first.PlanTaskID || !again.AlreadyExists {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if len(k.opener.opened) != 1 {
		t.Fatalf("a replay must not open a second approval, opened %d", len(k.opener.opened))
	}
}

func TestCommitPlan_ReplanSupersedesThePreviousPlan(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	if _, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)}); err != nil {
		t.Fatal(err)
	}
	old := k.w.requests[req.ID].PlanTaskID
	// A reviewer asks for changes: the Request is back in planning with the old Plan still recorded.
	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID, Feedback: "smaller tasks"}); err != nil {
		t.Fatal(err)
	}
	res, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil || res.PlanTaskID == old {
		t.Fatalf("replan: %+v %v", res, err)
	}
	if last := k.writer.calls[len(k.writer.calls)-1]; last.SupersedesPlanID != old {
		t.Fatalf("supersedes = %q, want %q", last.SupersedesPlanID, old)
	}
	if k.w.requests[req.ID].PlanTaskID != res.PlanTaskID {
		t.Fatal("the Request must point at the new Plan")
	}
}

func TestCommitPlan_StalePlanReferenceFallsBackToAPlainCreate(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	stale := k.w.requests[req.ID]
	stale.PlanTaskID = "11111111-1111-4111-8111-111111111111"
	k.w.requests[req.ID] = stale
	mismatch := domain.ErrPlanSupersedeMismatch()
	k.writer.errFn = func(in PlanTreeInput) error {
		if in.SupersedesPlanID != "" {
			return mismatch
		}
		return nil
	}
	res, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil || res.PlanTaskID == "" {
		t.Fatalf("commit: %+v %v", res, err)
	}
	if n := k.writer.callCount(); n != 2 || k.writer.calls[1].SupersedesPlanID != "" {
		t.Fatalf("want a retry without supersedes, calls=%d", n)
	}
}
