package usecase

import (
	"context"
	"testing"
)

func TestClone_DispatchesByReachability(t *testing.T) {
	t.Run("missing dev_server_id is rejected", func(t *testing.T) {
		uc := NewClone(&fakeDevServerReachability{allowLocal: true}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), CloneInput{})
		if err == nil {
			t.Fatal("expected error for missing dev_server_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{worktreePath: "/srv/repo", defaultBranch: "main"}
		local := &fakeGitExecutor{}
		uc := NewClone(reachability, local, relay)

		got, err := uc.Execute(context.Background(), CloneInput{DevServerID: "ds1", URL: "https://x", DestPath: "/srv/repo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.WorktreePath != "/srv/repo" || got.DefaultBranch != "main" {
			t.Errorf("unexpected result: %+v", got)
		}
		if !relay.calledClone || local.calledClone {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("unreachable dev server is rejected, never run locally", func(t *testing.T) {
		reachability := &fakeDevServerReachability{reachable: false, allowLocal: true}
		local := &fakeGitExecutor{worktreePath: "/local/repo", defaultBranch: "main"}
		relay := &fakeGitExecutor{}
		uc := NewClone(reachability, local, relay)

		_, err := uc.Execute(context.Background(), CloneInput{DevServerID: "ds1", URL: "https://x", DestPath: "/local/repo"})
		if ae := HostDispatchError(err); ae == nil || ae.Code != CodeDevServerUnreachable {
			t.Fatalf("expected %s, got %v", CodeDevServerUnreachable, err)
		}
		if local.calledClone || relay.calledClone {
			t.Error("expected no executor to be called")
		}
	})

}
