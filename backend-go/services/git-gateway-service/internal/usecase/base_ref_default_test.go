package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestBaseRefDefault_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewBaseRefDefault(&fakeDevServerReachability{allowLocal: true}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), BaseRefDefaultInput{})
		if err == nil {
			t.Fatal("expected error for missing repo_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{baseRef: "main"}
		local := &fakeGitExecutor{}
		uc := NewBaseRefDefault(reachability, projects, local, relay)

		ref, err := uc.Execute(context.Background(), BaseRefDefaultInput{RepoID: "repo-1"})
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

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		reachability := &fakeDevServerReachability{allowLocal: true}
		local := &fakeGitExecutor{baseRef: "main"}
		relay := &fakeGitExecutor{}
		uc := NewBaseRefDefault(reachability, projects, local, relay)

		_, err := uc.Execute(context.Background(), BaseRefDefaultInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledBaseRefDefault || relay.calledBaseRefDefault {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("repo not found is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewBaseRefDefault(&fakeDevServerReachability{allowLocal: true}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})

		_, err := uc.Execute(context.Background(), BaseRefDefaultInput{RepoID: "repo-1"})
		if err == nil {
			t.Fatal("expected error when GetRepo fails")
		}
	})

	t.Run("the project's configured default branch is passed to the executor as a hint", func(t *testing.T) {
		projects := &fakeProjectClient{
			getRepoResult:        domain.RepoInfo{ID: "repo-1", ProjectID: "proj-1", URL: "/local/repo"},
			projectDefaultBranch: "main",
		}
		local := &fakeGitExecutor{baseRef: "main"}
		uc := NewBaseRefDefault(&fakeDevServerReachability{allowLocal: true}, projects, local, &fakeGitExecutor{})

		if _, err := uc.Execute(context.Background(), BaseRefDefaultInput{RepoID: "repo-1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projects.gotDefaultBranchProjectID != "proj-1" || local.gotPreferredBranch != "main" {
			t.Errorf("project=%q preferred=%q", projects.gotDefaultBranchProjectID, local.gotPreferredBranch)
		}
	})

	t.Run("a failed project lookup degrades to no hint instead of failing", func(t *testing.T) {
		projects := &fakeProjectClient{
			getRepoResult:           domain.RepoInfo{ID: "repo-1", ProjectID: "proj-1", URL: "/local/repo"},
			projectDefaultBranchErr: context.DeadlineExceeded,
		}
		local := &fakeGitExecutor{baseRef: "main"}
		uc := NewBaseRefDefault(&fakeDevServerReachability{allowLocal: true}, projects, local, &fakeGitExecutor{})

		ref, err := uc.Execute(context.Background(), BaseRefDefaultInput{RepoID: "repo-1"})
		if err != nil || ref != "main" {
			t.Fatalf("ref=%q err=%v", ref, err)
		}
		if local.gotPreferredBranch != "" {
			t.Errorf("no hint expected, got %q", local.gotPreferredBranch)
		}
	})

	t.Run("a repo without a project id never calls the project lookup", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}, projectDefaultBranch: "main"}
		local := &fakeGitExecutor{baseRef: "main"}
		uc := NewBaseRefDefault(&fakeDevServerReachability{allowLocal: true}, projects, local, &fakeGitExecutor{})
		if _, err := uc.Execute(context.Background(), BaseRefDefaultInput{RepoID: "repo-1"}); err != nil {
			t.Fatal(err)
		}
		if projects.gotDefaultBranchProjectID != "" || local.gotPreferredBranch != "" {
			t.Errorf("lookup=%q preferred=%q", projects.gotDefaultBranchProjectID, local.gotPreferredBranch)
		}
	})
}
