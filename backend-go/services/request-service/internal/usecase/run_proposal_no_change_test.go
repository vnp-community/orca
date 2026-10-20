package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestProposalRun_NoChangeOnRevision_IsNoChangeError(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	first, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	revisionsBefore := len(k.proposal.revisions.rows)

	// The agent leaves every document as it was: the worktree has nothing to commit.
	eng.afterWrite = func(string) { k.proposal.ws.changed = nil }
	run2 := run
	run2.ID, run2.Feedback = "run-2", "make no change"
	k.w.runs[run2.ID] = run2
	f, ok := asFailure(w.Execute(k.w.ctx(), run2))
	if !ok || f.Code != "REQUEST_PLAN_REVISION_NO_CHANGE" || f.Message != "the agent did not change any document" {
		t.Fatalf("failure = %+v", f)
	}
	if f.Raw != "" {
		t.Fatalf("the failure carries no raw output: %q", f.Raw)
	}
	after, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	if after.Version != first.Version || after.Commit != first.Commit || len(k.proposal.revisions.rows) != revisionsBefore {
		t.Fatalf("a failed revision must leave the current version alone: %+v / %+v, %d revision rows", first, after, len(k.proposal.revisions.rows))
	}
	if got := k.w.requests[run.RequestID].Status; got != domain.RequestStatusPlanning {
		t.Fatalf("status = %s: the Request stays in planning", got)
	}
}

func TestProposalRun_FirstRunNothingToCommit_KeepsOldError(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	eng.afterWrite = func(string) { k.proposal.ws.changed = nil }
	f, ok := asFailure(w.Execute(k.w.ctx(), run))
	if !ok || f.Code != domain.RunErrSpecAgentFailed {
		t.Fatalf("a first run with nothing to commit keeps the generic code: %+v", f)
	}
}
