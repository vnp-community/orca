package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestSetWorktreeActivation_UpdatesActiveFlag(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main", Active: true}
	uc := NewSetWorktreeActivation(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, SetWorktreeActivationInput{WorktreeID: "w1", Active: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Active {
		t.Error("expected Active=false")
	}
}

func TestSetWorktreeActivation_NotFound(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewSetWorktreeActivation(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, SetWorktreeActivationInput{WorktreeID: "missing", Active: true})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND")
}
