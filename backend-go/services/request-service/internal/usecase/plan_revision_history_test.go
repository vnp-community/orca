package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestFinish_AppendsRevisionInSameTx(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	change, _, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	rev, found, err := k.proposal.revisions.GetByRun(context.Background(), run.ID)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if rev.Revision != 1 || rev.RequestID != run.RequestID || rev.Commit != change.Commit || rev.TasksMDDigest != change.TasksMDDigest ||
		rev.BundleDigest != change.BundleDigest() || !rev.TextKept || len(rev.Files) != len(change.Bundle) || len(rev.Files) == 0 {
		t.Fatalf("%+v vs change %+v", rev, change)
	}
	if k.w.runs[run.ID].Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %s", describe(k.w.runs[run.ID]))
	}
}

func TestFinish_FailedAppendFailsRun(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	k.proposal.revisions.err = errRevisionStore
	err := w.Execute(k.w.ctx(), run)
	if !errors.Is(err, errRevisionStore) {
		t.Fatalf("a version that cannot be stored must fail the run: %v", err)
	}
	if k.w.runs[run.ID].Status == domain.RunStatusSucceeded {
		t.Fatal("the run must not succeed without its version")
	}
	if len(k.proposal.revisions.rows) != 0 {
		t.Fatalf("no version may remain: %+v", k.proposal.revisions.rows)
	}
}

func TestFinish_RevisionNumberFollowsSucceededPlanRuns(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	base := time.Now().UTC().Add(-time.Hour)
	seed := func(kind domain.RunKind, status domain.RunStatus, at time.Duration) {
		id := uuid.NewString()
		k.w.runs[id] = domain.AnalysisRun{ID: id, RequestID: run.RequestID, Kind: kind, Status: status, StartedAt: base.Add(at)}
	}
	seed(domain.RunKindPlan, domain.RunStatusSucceeded, 0)
	seed(domain.RunKindPlan, domain.RunStatusFailed, time.Minute)
	seed(domain.RunKindSolution, domain.RunStatusSucceeded, 2*time.Minute)
	seed(domain.RunKindPlan, domain.RunStatusSucceeded, 3*time.Minute)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	rev, found, _ := k.proposal.revisions.GetByRun(context.Background(), run.ID)
	if !found || rev.Revision != 3 {
		t.Fatalf("two succeeded Plan runs came before: want version 3, got %+v found=%v", rev, found)
	}
}
