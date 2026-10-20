package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func wantErrCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestListRepoBranches(t *testing.T) {
	branches := []domain.RepoBranch{{Name: "main"}, {Name: "origin/hotfix/x", Remote: true}}

	t.Run("missing repo id is rejected", func(t *testing.T) {
		uc := NewListRepoBranches(&fakeDevServerReachability{allowLocal: true}, &fakeProjectClient{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ListRepoBranchesInput{})
		wantErrCode(t, err, "GITGATEWAY_MISSING_REPO_ID")
	})

	t.Run("reachable dev server dispatches to relay executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		relay := &fakeGitExecutor{repoBranches: branches, repoBranchesTruncated: true}
		local := &fakeGitExecutor{}
		uc := NewListRepoBranches(&fakeDevServerReachability{reachable: true}, projects, local, relay)

		got, err := uc.Execute(context.Background(), ListRepoBranchesInput{RepoID: "repo-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Branches) != 2 || !got.Truncated || got.Branches[1] != branches[1] {
			t.Errorf("got %+v", got)
		}
		if !relay.calledListRepoBranches || local.calledListRepoBranches {
			t.Error("expected relay executor to be called, not local")
		}
		if relay.gotRepoPath != "/srv/repo" {
			t.Errorf("executor repoPath = %q, want /srv/repo", relay.gotRepoPath)
		}
	})

	t.Run("no dev server bound dispatches to local executor", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		local := &fakeGitExecutor{repoBranches: branches}
		relay := &fakeGitExecutor{}
		uc := NewListRepoBranches(&fakeDevServerReachability{allowLocal: true}, projects, local, relay)

		if _, err := uc.Execute(context.Background(), ListRepoBranchesInput{RepoID: "repo-1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.calledListRepoBranches || relay.calledListRepoBranches {
			t.Error("expected local executor to be called, not relay")
		}
	})

	t.Run("unknown repo is not found", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoErr: context.DeadlineExceeded}
		uc := NewListRepoBranches(&fakeDevServerReachability{allowLocal: true}, projects, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ListRepoBranchesInput{RepoID: "repo-1"})
		wantErrCode(t, err, "WORKTREE_REPO_NOT_FOUND")
	})

	t.Run("unreachable host fails to resolve", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", DevServerID: "ds1", URL: "/srv/repo"}}
		relay := &fakeGitExecutor{}
		uc := NewListRepoBranches(&fakeDevServerReachability{reachable: false}, projects, &fakeGitExecutor{}, relay)
		_, err := uc.Execute(context.Background(), ListRepoBranchesInput{RepoID: "repo-1"})
		wantErrCode(t, err, "GITGATEWAY_RESOLVE_FAILED")
		if relay.calledListRepoBranches {
			t.Error("executor must not run when the host cannot be resolved")
		}
	})

	t.Run("executor failure is wrapped", func(t *testing.T) {
		projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/local/repo"}}
		local := &fakeGitExecutor{listRepoBranchesErr: errors.New("git exploded")}
		uc := NewListRepoBranches(&fakeDevServerReachability{allowLocal: true}, projects, local, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), ListRepoBranchesInput{RepoID: "repo-1"})
		wantErrCode(t, err, "GITGATEWAY_LIST_BRANCHES_FAILED")
	})
}
