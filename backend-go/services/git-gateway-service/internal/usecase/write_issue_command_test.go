package usecase

import (
	"context"
	"testing"
)

func TestWriteIssueCommand_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := NewWriteIssueCommand(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		err := uc.Execute(context.Background(), WriteIssueCommandInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/srv/repo"}}
		relay := &fakeGitExecutor{}
		local := &fakeGitExecutor{}
		uc := NewWriteIssueCommand(resolver, local, relay)

		if err := uc.Execute(context.Background(), WriteIssueCommandInput{WorktreeID: "wt1", Content: "x"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !relay.calledWriteIssueCommand || local.calledWriteIssueCommand {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/local/repo"}}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewWriteIssueCommand(resolver, local, relay)

		if err := uc.Execute(context.Background(), WriteIssueCommandInput{WorktreeID: "wt1", Content: "x"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledWriteIssueCommand || relay.calledWriteIssueCommand {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
