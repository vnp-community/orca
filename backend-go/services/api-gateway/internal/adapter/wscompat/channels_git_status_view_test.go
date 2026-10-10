package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

func i32(v int32) *int32 { return &v }

// The agent's rows keep staged/unstaged/untracked areas, rename source, line
// counts and the upstream/conflict summary; the view must hand them to the
// Source Control panel unchanged (G5c: it used to receive nothing at all).
func TestGitStatusChannel_PreservesAreasAndStatusSummary(t *testing.T) {
	fake := &fakeGitGatewayClient{
		getStatusFunc: func(_ context.Context, _ *gitgatewayv1.GetStatusRequest) (*gitgatewayv1.GetStatusResponse, error) {
			return &gitgatewayv1.GetStatusResponse{
				Branch: "refs/heads/task/abc", Head: "3824d41", ConflictOperation: "merge",
				Upstream: &gitgatewayv1.StatusUpstream{HasUpstream: true, UpstreamName: "origin/task/abc", Ahead: 2, Behind: 1},
				Files: []*gitgatewayv1.FileStatus{
					{Path: "src/calc2.py", State: "renamed", Area: "staged", OldPath: "src/calc.py", Added: i32(0), Removed: i32(0)},
					{Path: "tests/test_calc.py", State: "untracked", Area: "untracked", Added: i32(1)},
					{Path: "c.go", State: "conflicted", Area: "unstaged", ConflictKind: "both_modified", ConflictStatus: "unresolved"},
				},
			}, nil
		},
	}
	r := NewRegistry()
	registerGitChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "git.status",
		argsJSON(t, map[string]any{"worktree": "id:wt-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Entries  []map[string]any `json:"entries"`
		Head     string           `json:"head"`
		Branch   string           `json:"branch"`
		Conflict string           `json:"conflictOperation"`
		Upstream map[string]any   `json:"upstreamStatus"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}

	if wire.Branch != "refs/heads/task/abc" || wire.Head != "3824d41" || wire.Conflict != "merge" {
		t.Errorf("result summary lost: %s", raw)
	}
	if wire.Upstream["hasUpstream"] != true || wire.Upstream["upstreamName"] != "origin/task/abc" || wire.Upstream["ahead"] != float64(2) {
		t.Errorf("upstreamStatus lost: %v", wire.Upstream)
	}
	if len(wire.Entries) != 3 {
		t.Fatalf("entries = %s", raw)
	}
	renamed := wire.Entries[0]
	if renamed["area"] != "staged" || renamed["oldPath"] != "src/calc.py" || renamed["added"] != float64(0) || renamed["removed"] != float64(0) {
		t.Errorf("renamed row: %v", renamed)
	}
	untracked := wire.Entries[1]
	if untracked["area"] != "untracked" || untracked["added"] != float64(1) {
		t.Errorf("untracked row: %v", untracked)
	}
	if _, has := untracked["removed"]; has {
		t.Errorf("untracked row must not invent removed=0: %v", untracked)
	}
	conflict := wire.Entries[2]
	if conflict["status"] != "modified" || conflict["conflictKind"] != "both_modified" || conflict["conflictStatus"] != "unresolved" {
		t.Errorf("conflict row must read as modified + conflictKind: %v", conflict)
	}
}

func TestGitStatusChannel_HostLocalStatusGetsFrontendDefaults(t *testing.T) {
	fake := &fakeGitGatewayClient{
		getStatusFunc: func(_ context.Context, _ *gitgatewayv1.GetStatusRequest) (*gitgatewayv1.GetStatusResponse, error) {
			return &gitgatewayv1.GetStatusResponse{Files: []*gitgatewayv1.FileStatus{{Path: "n.txt", State: "untracked"}}}, nil
		},
	}
	r := NewRegistry()
	registerGitChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "git.status",
		argsJSON(t, map[string]any{"worktree": "id:wt-1"}))
	if err != nil {
		t.Fatal(err)
	}
	view := result.(gitStatusResultView)
	if view.ConflictOperation != "unknown" || view.UpstreamStatus != nil || view.Entries[0].Area != "untracked" {
		t.Errorf("unexpected defaults: %+v", view)
	}
}
