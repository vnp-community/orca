package domain

import "testing"

func TestStoredBaseRefLock(t *testing.T) {
	cases := []struct {
		name     string
		status   RequestStatus
		proposal bool
		want     BaseRefLockReason
	}{
		{"planning, nothing recorded", RequestStatusPlanning, false, BaseRefUnlocked},
		{"planning with a proposal worktree", RequestStatusPlanning, true, BaseRefLockedProposal},
		{"executing without a proposal", RequestStatusExecuting, false, BaseRefLockedExecution},
		{"executing with a proposal reports the proposal", RequestStatusExecuting, true, BaseRefLockedProposal},
		{"completed outranks a proposal", RequestStatusCompleted, true, BaseRefLockedTerminal},
		{"cancelled", RequestStatusCancelled, false, BaseRefLockedTerminal},
		{"backlog", RequestStatusRequestBacklog, false, BaseRefUnlocked},
	}
	for _, c := range cases {
		if got := StoredBaseRefLock(c.status, c.proposal); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	for want, raw := range map[BaseRefLockReason]string{BaseRefLockedProposal: "proposal", BaseRefLockedTask: "task", BaseRefLockedExecution: "execution", BaseRefLockedTerminal: "terminal"} {
		if string(want) != raw {
			t.Errorf("lock reason value %q changed: clients read it as %q", want, raw)
		}
	}
}
