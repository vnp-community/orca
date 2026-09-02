package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestSetWorktreeLineage_SetsParent(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["child"] = domain.Worktree{ID: "child", ProjectID: "p1", RepoID: "r1", Path: "/srv/child", Branch: "feature", Active: true}
	uc := NewSetWorktreeLineage(repo)

	parentID := "parent"
	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SetWorktreeLineageInput{WorktreeID: "child", ParentWorktreeID: &parentID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ParentWorktreeID == nil || *got.ParentWorktreeID != "parent" {
		t.Errorf("expected ParentWorktreeID=parent, got %v", got.ParentWorktreeID)
	}
}

func TestSetWorktreeLineage_ClearsParent(t *testing.T) {
	parentID := "parent"
	repo := newFakeWorktreeRepository()
	repo.worktrees["child"] = domain.Worktree{ID: "child", ProjectID: "p1", RepoID: "r1", Path: "/srv/child", Branch: "feature", Active: true, ParentWorktreeID: &parentID}
	uc := NewSetWorktreeLineage(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SetWorktreeLineageInput{WorktreeID: "child", ParentWorktreeID: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ParentWorktreeID != nil {
		t.Errorf("expected ParentWorktreeID cleared, got %v", got.ParentWorktreeID)
	}
}

func TestSetWorktreeLineage_RequiresWorktreeID(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewSetWorktreeLineage(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SetWorktreeLineageInput{})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED")
}

func TestSetWorktreeLineage_NotFound(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewSetWorktreeLineage(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SetWorktreeLineageInput{WorktreeID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND")
}
