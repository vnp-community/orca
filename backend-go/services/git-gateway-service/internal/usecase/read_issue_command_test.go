package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestReadIssueCommand_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewReadIssueCommand(&fakeDevServerReachability{}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ReadIssueCommandInput{})
		if err == nil {
			t.Fatal("expected error for missing repo_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{issueCommandContent: `{"cmd":"gh"}`, issueCommandExists: true}
		local := &fakeGitExecutor{}
		uc := NewReadIssueCommand(reachability, projects, local, relay)

		got, err := uc.Execute(context.Background(), ReadIssueCommandInput{RepoID: "repo-1"})
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

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		reachability := &fakeDevServerReachability{}
		local := &fakeGitExecutor{}
		relay := &fakeGitExecutor{}
		uc := NewReadIssueCommand(reachability, projects, local, relay)

		_, err := uc.Execute(context.Background(), ReadIssueCommandInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledReadIssueCommand || relay.calledReadIssueCommand {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("repo not found is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewReadIssueCommand(&fakeDevServerReachability{}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})

		_, err := uc.Execute(context.Background(), ReadIssueCommandInput{RepoID: "repo-1"})
		if err == nil {
			t.Fatal("expected error when GetRepo fails")
		}
	})
}
