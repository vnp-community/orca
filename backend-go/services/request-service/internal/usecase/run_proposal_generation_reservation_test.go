package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// The change row is read when the run starts, but ReservingProposalWorkspace may rewrite it before the worktree exists (an
// abandoned marker is reopened). Recording the worktree from the stale snapshot would put 'abandoned' back and drop the lock
// while a worktree exists.
func TestRunProposalGeneration_RecordsTheWorktreeOnTheReservedRowNotOnTheStartSnapshot(t *testing.T) {
	boom := errors.New("agent failed")
	eng := &scriptedEngine{announce: &Workspace{WorktreeID: "wt-9", Branch: "request/1-proposal"}, errs: []error{boom}}
	k, run, w := proposalKit(t, eng, nil)
	req := k.w.requests[run.RequestID]
	if _, err := k.proposal.changes.Upsert(context.Background(), domain.OpenSpecChange{
		RequestID: req.ID, ChangeID: "req-1-old", Branch: "request/1-proposal", Status: domain.ChangeStatusAbandoned, SyncState: domain.SyncStateInSync,
	}); err != nil {
		t.Fatal(err)
	}
	eng.reserve = func(ProposalInput) {
		c, _, _ := k.proposal.changes.GetByRequest(context.Background(), req.ID)
		if err := c.Reopen(); err != nil {
			t.Error(err)
		}
		_, _ = k.proposal.changes.Upsert(context.Background(), c)
	}
	if err := w.Execute(k.w.ctx(), run); err == nil {
		t.Fatal("the scripted engine fails the run")
	}
	got, _, _ := k.proposal.changes.GetByRequest(context.Background(), req.ID)
	if got.Status != domain.ChangeStatusPreparing || got.WorkspaceID != "wt-9" {
		t.Fatalf("the reserved row must keep its status and gain the worktree: %+v", got)
	}
}

func TestRunProposalGeneration_RecordsTheWorktreeWhenNoRowWasReserved(t *testing.T) {
	eng := &scriptedEngine{announce: &Workspace{WorktreeID: "wt-3", Branch: "request/1-proposal"}, errs: []error{errors.New("agent failed")}}
	k, run, w := proposalKit(t, eng, nil)
	_ = w.Execute(k.w.ctx(), run)
	got, found, _ := k.proposal.changes.GetByRequest(context.Background(), run.RequestID)
	if !found || got.Status != domain.ChangeStatusPreparing || got.WorkspaceID != "wt-3" || got.Branch != "request/1-proposal" {
		t.Fatalf("a failed run leaves the record the cleanup finds: found=%v %+v", found, got)
	}
}
