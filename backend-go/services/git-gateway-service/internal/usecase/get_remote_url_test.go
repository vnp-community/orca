package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestGetRemoteURL_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewGetRemoteURL(&fakeDevServerReachability{}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), GetRemoteURLInput{})
		if err == nil {
			t.Fatal("expected error for missing repo_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{remoteURL: "git@github.com:acme/widgets.git"}
		local := &fakeGitExecutor{}
		uc := NewGetRemoteURL(reachability, projects, local, relay)

		url, err := uc.Execute(context.Background(), GetRemoteURLInput{RepoID: "repo-1", RemoteName: "origin"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "git@github.com:acme/widgets.git" {
			t.Errorf("got url %q, want git@github.com:acme/widgets.git", url)
		}
		if !relay.calledRemoteURL || local.calledRemoteURL {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		reachability := &fakeDevServerReachability{}
		local := &fakeGitExecutor{remoteURL: "https://github.com/acme/widgets"}
		relay := &fakeGitExecutor{}
		uc := NewGetRemoteURL(reachability, projects, local, relay)

		_, err := uc.Execute(context.Background(), GetRemoteURLInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledRemoteURL || relay.calledRemoteURL {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("repo not found is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewGetRemoteURL(&fakeDevServerReachability{}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})

		_, err := uc.Execute(context.Background(), GetRemoteURLInput{RepoID: "repo-1"})
		if err == nil {
			t.Fatal("expected error when GetRepo fails")
		}
	})

	t.Run("remote not configured is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		local := &fakeGitExecutor{remoteURLErr: context.DeadlineExceeded}
		uc := NewGetRemoteURL(&fakeDevServerReachability{}, projects, local, &fakeGitExecutor{})

		_, err := uc.Execute(context.Background(), GetRemoteURLInput{RepoID: "repo-1", RemoteName: "upstream"})
		if err == nil {
			t.Fatal("expected error when the remote doesn't exist")
		}
	})
}
