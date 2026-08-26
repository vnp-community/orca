package usecase

import (
	"context"
	"testing"
)

func TestCheckHooks_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := NewCheckHooks(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), CheckHooksInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/srv/repo"}}
		relay := &fakeGitExecutor{installedHooks: []string{"pre-commit", "post-checkout"}, orcaHooksCurrent: true}
		local := &fakeGitExecutor{}
		uc := NewCheckHooks(resolver, local, relay)

		got, err := uc.Execute(context.Background(), CheckHooksInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.OrcaHooksCurrent {
			t.Errorf("unexpected result: %+v", got)
		}
		if !relay.calledCheckHooks || local.calledCheckHooks {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/local/repo"}}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewCheckHooks(resolver, local, relay)

		_, err := uc.Execute(context.Background(), CheckHooksInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledCheckHooks || relay.calledCheckHooks {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
