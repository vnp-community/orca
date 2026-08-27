package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRenameWorktree_UpdatesBranch(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "old-branch", Active: true}
	uc := NewRenameWorktree(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RenameWorktreeInput{WorktreeID: "w1", Branch: "new-branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Branch != "new-branch" {
		t.Errorf("expected Branch=new-branch, got %q", got.Branch)
	}
}

func TestRenameWorktree_RejectsEmptyBranch(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "old-branch", Active: true}
	uc := NewRenameWorktree(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RenameWorktreeInput{WorktreeID: "w1", Branch: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_BRANCH_REQUIRED")
}

func TestRenameWorktree_NotFound(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRenameWorktree(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RenameWorktreeInput{WorktreeID: "missing", Branch: "new-branch"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND")
}
