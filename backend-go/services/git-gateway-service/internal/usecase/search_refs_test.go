package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestSearchRefs_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewSearchRefs(&fakeDevServerReachability{}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), SearchRefsInput{})
		if err == nil {
			t.Fatal("expected error for missing repo_id")
		}
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		reachability := &fakeDevServerReachability{reachable: true}
		relay := &fakeGitExecutor{refs: []string{"main", "feature/x"}}
		local := &fakeGitExecutor{}
		uc := NewSearchRefs(reachability, projects, local, relay)

		refs, err := uc.Execute(context.Background(), SearchRefsInput{RepoID: "repo-1", Query: "feat"})
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

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		reachability := &fakeDevServerReachability{}
		local := &fakeGitExecutor{refs: []string{"main"}}
		relay := &fakeGitExecutor{}
		uc := NewSearchRefs(reachability, projects, local, relay)

		_, err := uc.Execute(context.Background(), SearchRefsInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledSearchRefs || relay.calledSearchRefs {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("repo not found is reported", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewSearchRefs(&fakeDevServerReachability{}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})

		_, err := uc.Execute(context.Background(), SearchRefsInput{RepoID: "repo-1"})
		if err == nil {
			t.Fatal("expected error when GetRepo fails")
		}
	})
}
