package usecase

import (
	"context"
	"testing"
)

func TestBaseRefDefault_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := NewBaseRefDefault(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), BaseRefDefaultInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/srv/repo"}}
		relay := &fakeGitExecutor{baseRef: "main"}
		local := &fakeGitExecutor{}
		uc := NewBaseRefDefault(resolver, local, relay)

		ref, err := uc.Execute(context.Background(), BaseRefDefaultInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref != "main" {
			t.Errorf("got ref %q, want main", ref)
		}
		if !relay.calledBaseRefDefault || local.calledBaseRefDefault {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/local/repo"}}
		local := &fakeGitExecutor{baseRef: "main"}
		relay := &fakeGitExecutor{}
		uc := NewBaseRefDefault(resolver, local, relay)

		_, err := uc.Execute(context.Background(), BaseRefDefaultInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledBaseRefDefault || relay.calledBaseRefDefault {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
