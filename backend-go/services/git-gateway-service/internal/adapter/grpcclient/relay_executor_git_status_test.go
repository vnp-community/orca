package grpcclient

import (
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// realAgentGitStatusResult is the verbatim `result` of the Dev Server Agent's
// git.status (agent getStatusOp, captured against a real linked worktree
// `<repo>-task-abc` on branch task/abc: a renamed+edited file, a file staged
// then edited again, and an untracked file). The gateway used to decode this
// into a {branch, files} shape the agent never sends, so `entries` was dropped
// and the Source Control panel of every relayed worktree stayed empty (G5c).
const realAgentGitStatusResult = `{"entries":[` +
	`{"path":"src/calc2.py","status":"renamed","area":"staged","oldPath":"src/calc.py","added":0,"removed":0},` +
	`{"path":"src/calc2.py","status":"modified","area":"unstaged","oldPath":"src/calc.py","added":2,"removed":0},` +
	`{"path":"staged.txt","status":"added","area":"staged","added":1,"removed":0},` +
	`{"path":"staged.txt","status":"modified","area":"unstaged","added":1,"removed":0},` +
	`{"path":"tests/test_calc.py","status":"untracked","area":"untracked","added":1}],` +
	`"conflictOperation":"unknown","head":"3824d4152da5fb38b8878141c328c312f3782336",` +
	`"branch":"refs/heads/task/abc","upstreamStatus":{"hasUpstream":false,"ahead":0,"behind":0}}`

func TestRelayExecutor_GetStatus_DecodesRealAgentEntries(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: realAgentGitStatusResult},
	}
	r := NewRelayExecutor(fake)

	status, err := r.GetStatus(ctxWithTenant(t), "/repo-task-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Branch != "refs/heads/task/abc" {
		t.Errorf("Branch = %q", status.Branch)
	}
	if len(status.Files) != 5 {
		t.Fatalf("expected the agent's 5 entries, got %d: %+v", len(status.Files), status.Files)
	}
}

func TestRelayExecutor_GetStatus_KeepsAreaOldPathLineStatsAndUpstream(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: realAgentGitStatusResult},
	}
	r := NewRelayExecutor(fake)

	status, err := r.GetStatus(ctxWithTenant(t), "/repo-task-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type row struct{ path, state, area, oldPath string }
	want := []row{
		{"src/calc2.py", "renamed", "staged", "src/calc.py"},
		{"src/calc2.py", "modified", "unstaged", "src/calc.py"},
		{"staged.txt", "added", "staged", ""},
		{"staged.txt", "modified", "unstaged", ""},
		{"tests/test_calc.py", "untracked", "untracked", ""},
	}
	for i, w := range want {
		f := status.Files[i]
		got := row{f.Path, string(f.State), f.Area, f.OldPath}
		if got != w {
			t.Errorf("row %d = %+v, want %+v", i, got, w)
		}
	}
	if untracked := status.Files[4]; untracked.Added == nil || *untracked.Added != 1 || untracked.Removed != nil {
		t.Errorf("untracked row must keep added=1 and no removed, got added=%v removed=%v", untracked.Added, untracked.Removed)
	}
	if status.Head != "3824d4152da5fb38b8878141c328c312f3782336" || status.ConflictOperation != "unknown" {
		t.Errorf("head/conflictOperation lost: %+v", status)
	}
	if status.Upstream == nil || status.Upstream.HasUpstream {
		t.Errorf("expected an upstream summary with hasUpstream=false, got %+v", status.Upstream)
	}
}

func TestRelayExecutor_GetStatus_UnmergedRowKeepsConflictKind(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"entries":[` +
		`{"path":"a.go","area":"unstaged","status":"modified","conflictKind":"both_modified","conflictStatus":"unresolved"}],` +
		`"conflictOperation":"merge","branch":"refs/heads/x","didHitLimit":true,"statusLength":9000}`}}
	r := NewRelayExecutor(fake)

	status, err := r.GetStatus(ctxWithTenant(t), "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := status.Files[0]
	if f.ConflictKind != "both_modified" || f.ConflictStatus != "unresolved" || status.ConflictOperation != "merge" {
		t.Errorf("conflict data lost: %+v / %q", f, status.ConflictOperation)
	}
	if !status.DidHitLimit || status.StatusLength != 9000 {
		t.Errorf("limit flags lost: %+v", status)
	}
}

func TestRelayExecutor_SubmoduleStatus_DecodesAgentEntries(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{relayResp: &infrafleetv1.RelayResponse{ResultJson: realAgentGitStatusResult}}
	r := NewRelayExecutor(fake)

	status, err := r.SubmoduleStatus(ctxWithTenant(t), "/repo", "vendor/sub", "unstaged")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status.Files) != 5 {
		t.Errorf("expected the agent's 5 entries, got %d", len(status.Files))
	}
}
