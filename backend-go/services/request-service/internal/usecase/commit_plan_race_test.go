package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Two committers read `planning`; the one that records second finds the Plan already recorded and must succeed.
func TestCommitPlan_LosingTheRecordRaceIsNotAnError(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	var winner CommitPlanResult
	k.commit.writer = &raceOnce{inner: k.writer, other: func(ctx context.Context) {
		res, err := k.commit.ExecuteForSystem(lcCtx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
		if err != nil {
			t.Errorf("winner: %v", err)
		}
		winner = res
	}}
	res, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	if err != nil {
		t.Fatalf("the loser of the race saw %v", err)
	}
	if res.PlanTaskID != winner.PlanTaskID || res.PlanTaskID == "" {
		t.Fatalf("loser plan %q != winner plan %q", res.PlanTaskID, winner.PlanTaskID)
	}
	if len(k.writer.trees) != 1 {
		t.Fatalf("task-service holds %d Plans", len(k.writer.trees))
	}
	if len(k.opener.opened) != 1 {
		t.Fatalf("an approval was opened %d times", len(k.opener.opened))
	}
	if n := countOutboxSubject(k.w.outboxSubjects(), domain.SubjectPlanGenerated); n != 1 {
		t.Fatalf("plan.generated emitted %d times", n)
	}
	if got := k.w.requests[req.ID]; got.Status != domain.RequestStatusAwaitingPlanApproval || got.PlanTaskID != res.PlanTaskID {
		t.Fatalf("request = %s plan=%q", got.Status, got.PlanTaskID)
	}
}

func TestCommitPlan_ADifferentRecordedPlanIsStillRefused(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	k.commit.writer = &raceOnce{inner: k.writer, other: func(context.Context) {
		r := k.w.requests[req.ID]
		r.Status, r.PlanTaskID = domain.RequestStatusAwaitingPlanApproval, "99999999-9999-4999-8999-999999999999"
		k.w.requests[req.ID] = r
	}}
	_, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_STATE_STALE")
}

func TestCommitPlan_ARequestThatLeftPlanningIsStillRefused(t *testing.T) {
	k, req := commitKit(t, domain.RequestTypeTask, domain.RequestSizeS)
	k.commit.writer = &raceOnce{inner: k.writer, other: func(context.Context) {
		r := k.w.requests[req.ID]
		r.Status = domain.RequestStatusCancelled
		k.w.requests[req.ID] = r
	}}
	_, err := k.commit.Execute(k.w.ctx(), CommitPlanInput{RequestID: req.ID, Proposal: proposalOf(t, false)})
	requireCode(t, err, "REQUEST_STATE_STALE")
}

// raceOnce runs `other` right after the first task-service call, i.e. between the read and the record.
type raceOnce struct {
	inner *fakePlanWriter
	other func(context.Context)
	done  bool
}

func (r *raceOnce) CreatePlanTree(ctx context.Context, in PlanTreeInput) (PlanTreeResult, error) {
	tree, err := r.inner.CreatePlanTree(ctx, in)
	if err == nil && !r.done && r.other != nil {
		r.done = true
		r.other(ctx)
	}
	return tree, err
}

func countOutboxSubject(subjects []string, want string) int {
	n := 0
	for _, s := range subjects {
		if s == want {
			n++
		}
	}
	return n
}
