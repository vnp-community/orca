package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestCommitPlanExecuteForSystem_CommitsWithoutACallerOrPermission(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	k.commit.auth = refusingPlanAuth{k.commit.auth}
	res, err := k.commit.ExecuteForSystem(lcCtx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil || res.PlanTaskID == "" {
		t.Fatalf("system commit: %+v %v", res, err)
	}
	got := k.w.requests[req.ID]
	if got.Status != domain.RequestStatusAwaitingPlanApproval || got.PlanTaskID != res.PlanTaskID {
		t.Fatalf("request = %s plan=%q", got.Status, got.PlanTaskID)
	}
	if call := k.writer.calls[0]; call.CreatorID != req.ReporterID {
		t.Fatalf("creator = %q, want the reporter %q", call.CreatorID, req.ReporterID)
	}
	if len(k.opener.opened) != 1 || k.opener.opened[0].RequestedBy != req.ReporterID {
		t.Fatalf("approval = %+v", k.opener.opened)
	}
}

func TestCommitPlanExecuteForSystem_ValidatesLikeThePublicPath(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM)
	_, err := k.commit.ExecuteForSystem(lcCtx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_PLAN_MIXED_CHILDREN")
	if k.writer.callCount() != 0 {
		t.Fatal("an invalid proposal must not reach task-service")
	}
}

func TestCommitPlanExecuteForSystem_AlreadyExistsStillSucceeds(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	// The browser committed first and task-service holds the Plan; the Request has not been updated yet.
	if _, err := k.writer.CreatePlanTree(lcCtx(), PlanTreeInput{Request: k.w.requests[req.ID], Proposal: proposalOf(t, false)}); err != nil {
		t.Fatal(err)
	}
	res, err := k.commit.ExecuteForSystem(lcCtx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil || !res.AlreadyExists {
		t.Fatalf("commit over an existing Plan: %+v %v", res, err)
	}
	if k.w.requests[req.ID].Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatal("the Request must still move to approval")
	}
}

func TestCommitPlanExecuteForSystem_NeedsAReporter(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	r := k.w.requests[req.ID]
	r.ReporterID = ""
	k.w.requests[req.ID] = r
	_, err := k.commit.ExecuteForSystem(lcCtx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_REPORTER_REQUIRED")
}

func TestCommitPlanExecute_StillNeedsACallerAndAuthorization(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	_, err := k.commit.Execute(lcCtx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_REPORTER_REQUIRED")
	k.commit.auth = refusingPlanAuth{k.commit.auth}
	_, err = k.commit.Execute(tenant.WithUserID(lcCtx(), "someone"), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
	if k.writer.callCount() != 0 {
		t.Fatal("a refused public call must not write")
	}
}

// Both engines end in the same stored form, so the server commits what an agent_proposal run produced.
func TestCommitPlanExecuteForSystem_CommitsTheRawOutputOfAnOpenSpecRun(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("run: %v", err)
	}
	done := k.w.runs[run.ID]
	if done.Status != domain.RunStatusSucceeded || done.RawOutput == nil {
		t.Fatalf("run = %s", describe(done))
	}
	proposal, err := DecodePlanProposal(*done.RawOutput)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	res, err := k.commit.ExecuteForSystem(lcCtx(), CommitPlanInput{RequestID: run.RequestID, Proposal: proposal})
	if err != nil || res.PlanTaskID == "" || len(res.TaskIDs) != 3 {
		t.Fatalf("commit: %+v %v", res, err)
	}
	if k.w.requests[run.RequestID].Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatal("the Request must be waiting for approval")
	}
	if k.writer.calls[0].Proposal.Phases[0].Tasks[0].Description == "" {
		t.Fatal("the task contract must reach task-service")
	}
}
