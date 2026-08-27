package usecase

import (
	"context"
	"testing"
)

func TestReadIssueCommand_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := NewReadIssueCommand(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ReadIssueCommandInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/srv/repo"}}
		relay := &fakeGitExecutor{issueCommandContent: `{"cmd":"gh"}`, issueCommandExists: true}
		local := &fakeGitExecutor{}
		uc := NewReadIssueCommand(resolver, local, relay)

		got, err := uc.Execute(context.Background(), ReadIssueCommandInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Exists {
			t.Errorf("unexpected result: %+v", got)
		}
		if !relay.calledReadIssueCommand || local.calledReadIssueCommand {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/local/repo"}}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewReadIssueCommand(resolver, local, relay)

		_, err := uc.Execute(context.Background(), ReadIssueCommandInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledReadIssueCommand || relay.calledReadIssueCommand {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
