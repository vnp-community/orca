package domain

import (
	"testing"
	"time"
)

func TestDecideProposalCleanup_Policy(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	fresh, old := now.Add(-time.Hour), now.Add(-8*24*time.Hour)
	day7 := DefaultProposalRetention
	cases := []struct {
		name    string
		change  ChangeStatus
		request RequestStatus
		updated time.Time
		active  bool
		want    ProposalCleanup
	}{
		{"a cancelled Request's change is abandoned and its worktree removed at once", ChangeStatusReady, RequestStatusCancelled, fresh, false, ProposalCleanup{Remove: true, Abandon: true}},
		{"the same, for a change still preparing", ChangeStatusPreparing, RequestStatusCancelled, fresh, false, ProposalCleanup{Remove: true, Abandon: true}},
		{"a run that is writing is never touched, even for a cancelled Request", ChangeStatusReady, RequestStatusCancelled, old, true, ProposalCleanup{}},
		{"a completed Request keeps the worktree for the archive step until the retention passes", ChangeStatusReady, RequestStatusCompleted, fresh, false, ProposalCleanup{}},
		{"then it goes, and the change keeps its status", ChangeStatusReady, RequestStatusCompleted, old, false, ProposalCleanup{Remove: true}},
		{"an archived change needs no worktree", ChangeStatusArchived, RequestStatusCompleted, fresh, false, ProposalCleanup{Remove: true}},
		{"a failed run's leftover is kept for a week", ChangeStatusPreparing, RequestStatusPlanning, fresh, false, ProposalCleanup{}},
		{"and removed after, without abandoning the change", ChangeStatusPreparing, RequestStatusPlanning, old, false, ProposalCleanup{Remove: true}},
		{"a ready change of an active Request is left alone however old", ChangeStatusReady, RequestStatusAwaitingPlanApproval, old, false, ProposalCleanup{}},
		{"an abandoned change that still holds a worktree loses it", ChangeStatusAbandoned, RequestStatusPlanning, fresh, false, ProposalCleanup{Remove: true}},
		{"a run in progress also protects a leftover", ChangeStatusPreparing, RequestStatusPlanning, old, true, ProposalCleanup{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DecideProposalCleanup(tc.change, tc.request, tc.updated, now, day7, tc.active); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestOpenSpecChange_ReleaseWorkspaceKeepsEverythingElse(t *testing.T) {
	c := OpenSpecChange{Branch: "request/1-proposal", Status: ChangeStatusReady, WorkspaceID: "wt-1", Bundle: map[string]string{"tasks.md": "x"}}
	c.ReleaseWorkspace()
	if c.WorkspaceID != "" || c.Branch != "request/1-proposal" || c.Status != ChangeStatusReady || len(c.Bundle) != 1 {
		t.Fatalf("%+v", c)
	}
}

func TestProposalBranchNamesOfARequest(t *testing.T) {
	id := "665542ee-cf81-49a6-ae16-fe1c9408c8cc"
	if got := ProposalBranchFor(1, id); got != "request/1-proposal-665542ee" {
		t.Fatalf("own branch %q", got)
	}
	for _, b := range []string{"request/1-proposal", "request/1-proposal-665542ee"} {
		if !IsProposalBranch(b, 1, id) {
			t.Fatalf("%q is a proposal branch of the Request", b)
		}
	}
	for _, b := range []string{"request/2-proposal", "request/1-proposal-deadbeef", "main"} {
		if IsProposalBranch(b, 1, id) {
			t.Fatalf("%q is not a proposal branch of the Request", b)
		}
	}
	if TestScriptBranch(1, id, "TDD NOTES", 2) != "tsa/REQ-1-665542ee-TDD-NOTES-v2" || RequestIDShort("ab-c") != "abc" {
		t.Fatal("test-script branch or short id")
	}
}
