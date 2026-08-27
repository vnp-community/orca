package usecase

import (
	"context"
	"testing"
)

func TestSearchRefs_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := NewSearchRefs(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), SearchRefsInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/srv/repo"}}
		relay := &fakeGitExecutor{refs: []string{"main", "feature/x"}}
		local := &fakeGitExecutor{}
		uc := NewSearchRefs(resolver, local, relay)

		refs, err := uc.Execute(context.Background(), SearchRefsInput{WorktreeID: "wt1", Query: "feat"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(refs) != 2 {
			t.Errorf("got refs %+v", refs)
		}
		if !relay.calledSearchRefs || local.calledSearchRefs {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/local/repo"}}
		local := &fakeGitExecutor{refs: []string{"main"}}
		relay := &fakeGitExecutor{}
		uc := NewSearchRefs(resolver, local, relay)

		_, err := uc.Execute(context.Background(), SearchRefsInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledSearchRefs || relay.calledSearchRefs {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
