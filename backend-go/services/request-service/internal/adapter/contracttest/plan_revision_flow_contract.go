package contracttest

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RunPlanRevisionFlowContract is what both dialects must do when people ask for changes to a Plan that the server wrote (CR-REQ-045):
// each ask cancels the open Approval, starts one run and one `revision` job, the worker commits the new Plan with no client commit, and
// the versions read back in order with the ask that made each.
func RunPlanRevisionFlowContract(t *testing.T, env PlanAutoFlowEnv) {
	t.Helper()
	t.Run("ThreeRevisionsAreCommittedByTheWorkerAndEachCancelsTheOpenApproval", func(t *testing.T) { flowThreeRevisions(t, env) })
}

func flowThreeRevisions(t *testing.T, env PlanAutoFlowEnv) {
	k := newFlowKit(t, env, 0)
	ft := k.seed()
	w := k.worker("w1")
	k.deliver(ft, k.event(ft, uuid.NewString()))
	k.until(w, "the first run to start", func() bool { return k.job(ft).State == domain.PlanAutoRunning })
	k.finishRun(ft, k.planRuns(ft)[0].ID, "")
	k.requireDone(ft, w)

	asks := []string{"split the work", "add a rollback task", "reorder the risky part first"}
	planIDs := []string{k.reload(ft).PlanTaskID}
	for round, ask := range asks {
		cancelsBefore := len(k.canceler.all())
		started, err := k.generate.Execute(ft.ctx, usecase.GeneratePlanInput{RequestID: ft.req.ID, Feedback: ask, IdempotencyKey: fmt.Sprintf("revise-%d", round)})
		if err != nil || !started.Created {
			t.Fatalf("round %d: %+v %v", round+1, started, err)
		}
		if got := k.canceler.all(); len(got) != cancelsBefore+1 || got[len(got)-1] != "revision_requested" {
			t.Fatalf("round %d: the open approval was not cancelled for the revision: %v", round+1, got)
		}
		if req := k.reload(ft); req.Status != domain.RequestStatusPlanning || req.PlanTaskID != planIDs[len(planIDs)-1] {
			t.Fatalf("round %d: while the run goes the Request is %s with Plan %q (the old Plan stays until the new one is committed)", round+1, req.Status, req.PlanTaskID)
		}
		job := k.job(ft)
		if job.Origin != domain.PlanAutoOriginRevision || job.State != domain.PlanAutoRunning || job.RunID != started.RunID || job.MaxAttempts != 1 {
			t.Fatalf("round %d: the revision job is %+v", round+1, job)
		}
		k.finishRun(ft, started.RunID, "")
		k.requireDone(ft, w)
		fresh := k.reload(ft).PlanTaskID
		if fresh == planIDs[len(planIDs)-1] {
			t.Fatalf("round %d: the Request still points at the Plan that was revised", round+1)
		}
		planIDs = append(planIDs, fresh)
		if k.opener.count(ft.req.ID) != round+2 {
			t.Fatalf("round %d: %d approvals opened in all, want one per version", round+1, k.opener.count(ft.req.ID))
		}
	}
	if got := k.env.CountRows(t, "plan_auto_jobs", ft.tenantID); got != 4 {
		t.Fatalf("%d jobs for a Plan and three revisions", got)
	}
	if k.writer.calls < 4 {
		t.Fatalf("task-service was asked %d times", k.writer.calls)
	}

	list := usecase.NewListPlanRevisions(env.Requests, env.AnalysisRuns, nil, nil)
	page, err := list.Execute(ft.ctx, ft.req.ID, 0, "")
	if err != nil || len(page.Revisions) != 4 || page.NextPageToken != "" {
		t.Fatalf("versions: %+v %v", page, err)
	}
	for i, v := range page.Revisions { // newest first
		number := 4 - i
		wantAsk := ""
		if number > 1 {
			wantAsk = asks[number-2]
		}
		wantStatus := "superseded"
		if number == 4 {
			wantStatus = "current"
		}
		if v.Revision != number || v.Status != wantStatus || v.RevisionFeedback != wantAsk || v.ActorID != ft.userID {
			t.Fatalf("version %d = %+v, want status %s and ask %q", number, v, wantStatus, wantAsk)
		}
	}
	first, err := list.Execute(ft.ctx, ft.req.ID, 2, "")
	if err != nil || len(first.Revisions) != 2 || first.NextPageToken == "" || first.Revisions[0].Revision != 4 {
		t.Fatalf("first page of two: %+v %v", first, err)
	}
	second, err := list.Execute(ft.ctx, ft.req.ID, 2, first.NextPageToken)
	if err != nil || len(second.Revisions) != 2 || second.NextPageToken != "" || second.Revisions[1].Revision != 1 {
		t.Fatalf("second page of two: %+v %v", second, err)
	}
	read := usecase.NewGetPlanProposal(env.Requests, env.AnalysisRuns)
	for number := 1; number <= 4; number++ {
		view, err := read.Execute(ft.ctx, ft.req.ID, "", number)
		if err != nil || view.Proposal == nil || view.Status != "succeeded" {
			t.Fatalf("proposal of version %d: %+v %v", number, view, err)
		}
	}
}
