package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestGeneratePlan_StartsAPlanRunWithoutADraftSolution(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	before := len(k.w.solutions)

	res, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
	if err != nil || !res.Created || res.RunID == "" {
		t.Fatalf("generate: %+v %v", res, err)
	}
	run := k.w.runs[res.RunID]
	if run.Kind != domain.RunKindPlan || run.Mode != domain.AnalysisModeComplete || run.Status != domain.RunStatusRunning || run.SolutionID != "" {
		t.Fatalf("run = %s", describe(run))
	}
	if len(k.w.solutions) != before {
		t.Fatal("a plan run must not create a Solution")
	}
	if len(k.spawner.runs) != 1 || k.spawner.runs[0].ID != res.RunID {
		t.Fatalf("the run was not handed to the runner: %+v", k.spawner.runs)
	}
	if k.writer.callCount() != 0 {
		t.Fatal("PROPOSE must not write to task-service")
	}
}

func TestGeneratePlan_SecondCallReturnsTheRunningRun(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	first, _ := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
	second, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
	if err != nil || second.Created || second.RunID != first.RunID {
		t.Fatalf("second call: %+v %v", second, err)
	}
	if len(k.spawner.runs) != 1 {
		t.Fatalf("a duplicate call must not start a second worker, got %d", len(k.spawner.runs))
	}
}

func TestGeneratePlan_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		typ    domain.RequestType
		status domain.RequestStatus
		solved bool
		conns  AnalysisConnectionResolver
		want   string
	}{
		{"wrong state", domain.RequestTypeTask, domain.RequestStatusAnalyzing, true, fakeConns{}, "REQUEST_STATE_STALE"},
		{"type without a plan", domain.RequestTypeSpike, domain.RequestStatusPlanning, true, fakeConns{}, "REQUEST_PLAN_NOT_APPLICABLE"},
		{"solution not approved", domain.RequestTypeChangeRequest, domain.RequestStatusPlanning, false, fakeConns{}, "REQUEST_PLAN_SOLUTION_NOT_APPROVED"},
		{"no dev server", domain.RequestTypeTask, domain.RequestStatusPlanning, true, fakeConns{err: ErrNoDevServer}, "REQUEST_PLAN_AI_UNAVAILABLE"},
	}
	for _, c := range cases {
		k := newPlanKit(c.conns)
		req := k.w.seedRequest(c.typ, c.status)
		if c.solved {
			k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
		}
		_, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
		if !errorHasCode(err, c.want) {
			t.Errorf("%s: got %v, want %s", c.name, err, c.want)
		}
		if len(k.w.runs) != 0 {
			t.Errorf("%s: a refused call left %d run(s) behind", c.name, len(k.w.runs))
		}
	}
}

func TestGeneratePlan_RegenerateWithFeedbackCancelsTheApprovalAndReopensPlanning(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingPlanApproval)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)

	if _, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID}); !errorHasCode(err, "REQUEST_STATE_STALE") {
		t.Fatalf("without feedback a waiting Plan cannot be regenerated: %v", err)
	}
	res, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID, Feedback: "split the migration"})
	if err != nil || !res.Created {
		t.Fatalf("regenerate: %+v %v", res, err)
	}
	if got := k.w.requests[req.ID].Status; got != domain.RequestStatusPlanning {
		t.Fatalf("status = %s, want planning", got)
	}
	if len(k.w.cancelled) != 1 {
		t.Fatalf("pending approval not cancelled: %v", k.w.cancelled)
	}
	if k.w.runs[res.RunID].Feedback != "split the migration" {
		t.Fatal("the reviewer's feedback must reach the run")
	}
}

func TestGeneratePlan_FeedbackLimit(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	long := make([]rune, maxFeedbackRunes+1)
	for i := range long {
		long[i] = 'x'
	}
	_, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID, Feedback: string(long)})
	requireCode(t, err, "REQUEST_PLAN_FEEDBACK_TOO_LONG")
}
