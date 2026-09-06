package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestWriteIssueCommand_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewWriteIssueCommand(&fakeDevServerReachability{}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		err := uc.Execute(context.Background(), WriteIssueCommandInput{})
		if err == nil {
			t.Fatal("expected error for missing repo_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{}
		local := &fakeGitExecutor{}
		uc := NewWriteIssueCommand(reachability, projects, local, relay)

		if err := uc.Execute(context.Background(), WriteIssueCommandInput{RepoID: "repo-1", Content: "x"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !relay.calledWriteIssueCommand || local.calledWriteIssueCommand {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		reachability := &fakeDevServerReachability{}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewWriteIssueCommand(reachability, projects, local, relay)

		if err := uc.Execute(context.Background(), WriteIssueCommandInput{RepoID: "repo-1", Content: "x"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledWriteIssueCommand || relay.calledWriteIssueCommand {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("repo not found is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewWriteIssueCommand(&fakeDevServerReachability{}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})

		err := uc.Execute(context.Background(), WriteIssueCommandInput{RepoID: "repo-1"})
		if err == nil {
			t.Fatal("expected error when GetRepo fails")
		}
	})
}
