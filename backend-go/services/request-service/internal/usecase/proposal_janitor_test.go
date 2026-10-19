package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var janitorNow = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

type oneTenant struct{ ids []string }

func (o oneTenant) ListTenants(context.Context) ([]string, error) { return o.ids, nil }

type janitorKit struct {
	w       *solWorld
	changes *fakeChanges
	ws      *fakeWorkspace
	j       *ProposalJanitor
}

func newJanitorKit() *janitorKit {
	w := newSolWorld()
	k := &janitorKit{w: w, changes: newFakeChanges(), ws: newFakeWorkspace()}
	k.j = &ProposalJanitor{
		Changes: k.changes, Requests: w, Runs: fakeRuns{w}, Workspace: k.ws, Tenants: oneTenant{ids: []string{"t1"}},
		Now: func() time.Time { return janitorNow },
	}
	return k
}

// hold seeds a Request (number 42, so its branch is request/42-proposal) whose change holds a worktree.
func (k *janitorKit) hold(status domain.RequestStatus, change domain.ChangeStatus, age time.Duration) domain.Request {
	req := k.w.seedRequest(domain.RequestTypeChangeRequest, status)
	r := k.w.requests[req.ID]
	r.Number = 42
	k.w.requests[req.ID] = r
	k.changes.byReq[req.ID] = domain.OpenSpecChange{ID: "c-" + req.ID, RequestID: req.ID, ChangeID: "req-42-x", Branch: "request/42-proposal", Status: change, SyncState: domain.SyncStateInSync, WorkspaceID: "wt-1", Version: 1}
	k.changes.status[req.ID] = status
	k.changes.updated[req.ID] = janitorNow.Add(-age)
	return r
}

func (k *janitorKit) planRun(req domain.Request, status domain.RunStatus) {
	k.w.runs["run-"+req.ID] = domain.AnalysisRun{ID: "run-" + req.ID, RequestID: req.ID, Kind: domain.RunKindPlan, Status: status, StartedAt: janitorNow}
}

func TestProposalJanitor_ACancelledRequestsWorktreeGoesAndItsChangeIsAbandonedButTheBranchStays(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusCancelled, domain.ChangeStatusReady, time.Minute)
	sum, err := k.j.Sweep(context.Background())
	if err != nil || sum.Removed != 1 || sum.Abandoned != 1 || k.ws.removed != 1 {
		t.Fatalf("sum=%+v err=%v removed=%d", sum, err, k.ws.removed)
	}
	got := k.changes.byReq[req.ID]
	if got.WorkspaceID != "" || got.Status != domain.ChangeStatusAbandoned || got.Branch != "request/42-proposal" || got.ChangeID != "req-42-x" {
		t.Fatalf("the change keeps its branch and only loses the worktree: %+v", got)
	}
	if again, _ := k.j.Sweep(context.Background()); again.Removed != 0 || k.ws.removed != 1 {
		t.Fatalf("a second pass removed again: %+v", again)
	}
}

func TestProposalJanitor_NeverTouchesAWorktreeARunIsWritingIn(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusCancelled, domain.ChangeStatusPreparing, 30*24*time.Hour)
	k.planRun(req, domain.RunStatusRunning)
	if sum, _ := k.j.Sweep(context.Background()); sum.Removed != 0 || k.ws.removed != 0 {
		t.Fatalf("a running run's worktree was removed: %+v", sum)
	}
	k.planRun(req, domain.RunStatusFailed) // the run ended
	if sum, _ := k.j.Sweep(context.Background()); sum.Removed != 1 {
		t.Fatalf("%+v", sum)
	}
}

func TestProposalJanitor_AFailedRunsLeftoverIsKeptForTheRetentionAndNotAbandoned(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusPlanning, domain.ChangeStatusPreparing, 6*24*time.Hour)
	if sum, _ := k.j.Sweep(context.Background()); sum.Removed != 0 {
		t.Fatalf("removed before the retention: %+v", sum)
	}
	k.changes.updated[req.ID] = janitorNow.Add(-7 * 24 * time.Hour)
	sum, _ := k.j.Sweep(context.Background())
	got := k.changes.byReq[req.ID]
	if sum.Removed != 1 || sum.Abandoned != 0 || got.Status != domain.ChangeStatusPreparing || got.WorkspaceID != "" {
		t.Fatalf("sum=%+v change=%+v: the Request may still retry, so the change stays preparing", sum, got)
	}
	k.j.Retention = 30 * 24 * time.Hour // the retention is configurable
	k2 := newJanitorKit()
	k2.j.Retention = 30 * 24 * time.Hour
	k2.hold(domain.RequestStatusPlanning, domain.ChangeStatusPreparing, 10*24*time.Hour)
	if sum, _ := k2.j.Sweep(context.Background()); sum.Removed != 0 {
		t.Fatalf("a longer retention was ignored: %+v", sum)
	}
}

func TestProposalJanitor_ACompletedRequestsWorktreeWaitsForTheArchiveThenGoes(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusCompleted, domain.ChangeStatusReady, time.Hour)
	if sum, _ := k.j.Sweep(context.Background()); sum.Removed != 0 {
		t.Fatalf("%+v", sum)
	}
	c := k.changes.byReq[req.ID]
	c.Status = domain.ChangeStatusArchived
	k.changes.byReq[req.ID] = c
	if sum, _ := k.j.Sweep(context.Background()); sum.Removed != 1 || k.changes.byReq[req.ID].Status != domain.ChangeStatusArchived {
		t.Fatalf("%+v change=%+v", sum, k.changes.byReq[req.ID])
	}
}

func TestProposalJanitor_RefusesToRemoveAWorktreeThatIsNotTheRequestsOwnBranch(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusCancelled, domain.ChangeStatusReady, time.Minute)
	c := k.changes.byReq[req.ID]
	c.Branch = "main"
	k.changes.byReq[req.ID] = c
	sum, _ := k.j.Sweep(context.Background())
	if sum.Removed != 0 || sum.Failed != 1 || k.ws.removed != 0 || k.changes.byReq[req.ID].WorkspaceID != "wt-1" {
		t.Fatalf("sum=%+v removed=%d: a worktree on another branch must be left alone", sum, k.ws.removed)
	}
}

func TestProposalJanitor_AFailingRemoveKeepsTheRecordSoTheNextPassRetries(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusCancelled, domain.ChangeStatusReady, time.Minute)
	failing := &removeFails{fakeWorkspace: k.ws}
	k.j.Workspace = failing
	if sum, _ := k.j.Sweep(context.Background()); sum.Failed != 1 || sum.Removed != 0 || k.changes.byReq[req.ID].WorkspaceID != "wt-1" || k.changes.byReq[req.ID].Status != domain.ChangeStatusReady {
		t.Fatalf("%+v change=%+v", sum, k.changes.byReq[req.ID])
	}
	k.j.Workspace = k.ws
	if sum, _ := k.j.Sweep(context.Background()); sum.Removed != 1 {
		t.Fatalf("%+v", sum)
	}
}

type removeFails struct{ *fakeWorkspace }

func (r *removeFails) Remove(context.Context, Workspace) error { return errors.New("git-gateway down") }

func TestProposalJanitor_CleanupByHandRemovesEvenAnActiveRequestsWorktreeButNotARunning(t *testing.T) {
	k := newJanitorKit()
	req := k.hold(domain.RequestStatusAwaitingPlanApproval, domain.ChangeStatusReady, time.Minute)
	k.planRun(req, domain.RunStatusRunning)
	if _, err := k.j.Cleanup(context.Background(), req.ID); !errors.Is(err, ErrProposalRunActive) || k.ws.removed != 0 {
		t.Fatalf("err=%v removed=%d", err, k.ws.removed)
	}
	k.planRun(req, domain.RunStatusSucceeded)
	rel, err := k.j.Cleanup(context.Background(), req.ID)
	if err != nil || !rel.Removed || rel.Abandoned || rel.Branch != "request/42-proposal" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
	if got := k.changes.byReq[req.ID]; got.Status != domain.ChangeStatusReady || got.WorkspaceID != "" {
		t.Fatalf("the plan stays reviewable: %+v", got)
	}
	if rel, err := k.j.Cleanup(context.Background(), req.ID); err != nil || rel.Removed {
		t.Fatalf("nothing left to remove: %+v %v", rel, err)
	}
	none := k.w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	if rel, err := k.j.Cleanup(context.Background(), none.ID); err != nil || rel.Removed {
		t.Fatalf("a Request without a change: %+v %v", rel, err)
	}
}

type recordingReleaser struct{ calls []string }

func (r *recordingReleaser) ReleaseOnCancel(_ context.Context, id string) error {
	r.calls = append(r.calls, id)
	return nil
}

func TestCancelRequest_ReleasesTheProposalOnceAndAFailureNeverUndoesTheCancel(t *testing.T) {
	l := newLcLifecycle()
	rel := &recordingReleaser{}
	l.cancel.WithProposalCleanup(rel)
	r := l.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusAwaitingPlanApproval })
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: "dup", ActorID: "u1"}); err != nil || len(rel.calls) != 1 || rel.calls[0] != r.ID {
		t.Fatalf("err=%v calls=%v", err, rel.calls)
	}
	// A repeated cancel changes nothing, so it cleans nothing.
	if _, err := l.cancel.Execute(lcCtx(), CancelInput{RequestID: r.ID, Reason: "dup", ActorID: "u1"}); err != nil || len(rel.calls) != 1 {
		t.Fatalf("err=%v calls=%v", err, rel.calls)
	}
	l2 := newLcLifecycle()
	l2.cancel.WithProposalCleanup(failingReleaser{})
	r2 := l2.s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusPlanning })
	res, err := l2.cancel.Execute(lcCtx(), CancelInput{RequestID: r2.ID, Reason: "dup", ActorID: "u1"})
	if err != nil || res.Request.Status != domain.RequestStatusCancelled {
		t.Fatalf("res=%+v err=%v", res.Request.Status, err)
	}
}

type failingReleaser struct{}

func (failingReleaser) ReleaseOnCancel(context.Context, string) error { return errors.New("down") }

func TestRunProposalGeneration_AFailedRunLeavesAPreparingRowThatNamesItsWorktree(t *testing.T) {
	eng := &scriptedEngine{errs: []error{errors.New("agent failed")}, answers: []ProposalResult{{}}, announce: &Workspace{WorktreeID: "wt-9", Branch: "request/1-proposal"}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err == nil {
		t.Fatal("the run was meant to fail")
	}
	got, found, _ := k.proposal.changes.GetByRequest(k.w.ctx(), run.RequestID)
	if !found || got.Status != domain.ChangeStatusPreparing || got.WorkspaceID != "wt-9" || !domain.ValidChangeID(got.ChangeID) || got.Branch != "request/1-proposal" {
		t.Fatalf("the failed run left %+v (found=%v): the cleanup could not find its worktree", got, found)
	}
	// A retry of the same Request reuses the change id the failed run fixed.
	if inputs := len(eng.inputs); inputs != 1 || eng.inputs[0].ChangeID != got.ChangeID {
		t.Fatalf("inputs=%d change=%s", inputs, got.ChangeID)
	}
}

func TestRunProposalGeneration_ASuccessfulRunStoresTheWorktreeOnTheReadyChange(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	got, _, _ := k.proposal.changes.GetByRequest(k.w.ctx(), run.RequestID)
	if got.Status != domain.ChangeStatusReady || got.WorkspaceID != k.proposal.ws.ws.WorktreeID {
		t.Fatalf("%+v", got)
	}
}
