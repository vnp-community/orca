package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestDetectWorktrees_NoDevServer_DispatchesLocalUsingRepoURL(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{listWorktreePathsOut: []domain.WorktreeGitInfo{{Path: "/repo"}, {Path: "/repo-feature"}}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/repo"}}
	uc := NewDetectWorktrees(reachability, projects, local, relay)

	got, err := uc.Execute(context.Background(), "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Path != "/repo" {
		t.Errorf("unexpected result: %+v", got)
	}
	if !local.calledListWorktreePaths || relay.calledListWorktreePaths {
		t.Error("expected the local executor to be used when the repo has no dev server bound")
	}
	if local.gotRepoPath != "/repo" {
		t.Errorf("expected repo.URL to be used as repoPath, got %q", local.gotRepoPath)
	}
}

func TestDetectWorktrees_ReachableDevServer_DispatchesRelay(t *testing.T) {
	reachability := &fakeDevServerReachability{reachable: true}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{listWorktreePathsOut: []domain.WorktreeGitInfo{{Path: "/srv/repo"}}}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/srv/repo", DevServerID: "ds-1"}}
	uc := NewDetectWorktrees(reachability, projects, local, relay)

	got, err := uc.Execute(context.Background(), "repo-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/srv/repo" {
		t.Errorf("unexpected result: %+v", got)
	}
	if !relay.calledListWorktreePaths || local.calledListWorktreePaths {
		t.Error("expected the relay executor to be used when the repo's dev server is reachable")
	}
}

func TestDetectWorktrees_UnreachableDevServer_DispatchesLocal(t *testing.T) {
	reachability := &fakeDevServerReachability{reachable: false}
	local := &fakeGitExecutor{listWorktreePathsOut: []domain.WorktreeGitInfo{{Path: "/srv/repo"}}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/srv/repo", DevServerID: "ds-1"}}
	uc := NewDetectWorktrees(reachability, projects, local, relay)

	if _, err := uc.Execute(context.Background(), "repo-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledListWorktreePaths || relay.calledListWorktreePaths {
		t.Error("expected the local executor when the dev server is bound but not reachable")
	}
}

func TestDetectWorktrees_RepoNotFound(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoErr: errors.New("repo not found")}
	uc := NewDetectWorktrees(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), "missing")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound || ae.Code != "WORKTREE_REPO_NOT_FOUND" {
		t.Fatalf("expected WORKTREE_REPO_NOT_FOUND (KindNotFound), got %v", err)
	}
	if local.calledListWorktreePaths || relay.calledListWorktreePaths {
		t.Error("expected no executor call when the repo lookup fails")
	}
}

func TestDetectWorktrees_ReachabilityCheckFails(t *testing.T) {
	reachability := &fakeDevServerReachability{err: errors.New("infra-fleet-service unreachable")}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/repo", DevServerID: "ds-1"}}
	uc := NewDetectWorktrees(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), "repo-1")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_RESOLVE_FAILED" {
		t.Fatalf("expected WORKTREE_RESOLVE_FAILED, got %v", err)
	}
}

func TestDetectWorktrees_ListWorktreePathsFails(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{listWorktreePathsErr: errors.New("git worktree list: exit status 128")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{getRepoResult: domain.RepoInfo{ID: "repo-1", URL: "/repo"}}
	uc := NewDetectWorktrees(reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), "repo-1")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_DETECT_FAILED" {
		t.Fatalf("expected WORKTREE_DETECT_FAILED, got %v", err)
	}
}
