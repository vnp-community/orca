package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestRecordWorktreeRemoved_DeletesWorktree(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main", Active: true}
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: "w1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.worktrees["w1"]; ok {
		t.Error("expected worktree to be hard-deleted")
	}
}

func TestRecordWorktreeRemoved_NotFound(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: "missing"})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND")
}

func TestRecordWorktreeRemoved_RequiresWorktreeID(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeRemoved(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, RecordWorktreeRemovedInput{WorktreeID: ""})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED")
}
