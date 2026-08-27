package usecase

import (
	"context"
	"testing"
)

func TestInitRepo_DispatchesByReachability(t *testing.T) {
	t.Run("missing dev_server_id is rejected", func(t *testing.T) {
		uc := NewInitRepo(&fakeDevServerReachability{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), InitRepoInput{})
		if err == nil {
			t.Fatal("expected error for missing dev_server_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{initPath: "/srv/repo", defaultBranch: "main"}
		local := &fakeGitExecutor{}
		uc := NewInitRepo(reachability, local, relay)

		got, err := uc.Execute(context.Background(), InitRepoInput{DevServerID: "ds1", DestPath: "/srv/repo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "/srv/repo" {
			t.Errorf("unexpected result: %+v", got)
		}
		if !relay.calledInitRepo || local.calledInitRepo {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("unreachable dev server dispatches to local executor", func(t *testing.T) {
		reachability := &fakeDevServerReachability{reachable: false}
		local := &fakeGitExecutor{initPath: "/local/repo", defaultBranch: "main"}
		relay := &fakeGitExecutor{}
		uc := NewInitRepo(reachability, local, relay)

		_, err := uc.Execute(context.Background(), InitRepoInput{DevServerID: "ds1", DestPath: "/local/repo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledInitRepo || relay.calledInitRepo {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
