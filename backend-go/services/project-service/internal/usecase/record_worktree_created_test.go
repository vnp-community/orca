package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestRecordWorktreeCreated_PersistsAndStartsActive(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, RecordWorktreeCreatedInput{
		ProjectID: "p1", RepoID: "r1", Path: "/srv/worktrees/w1", Branch: "feature/x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Active {
		t.Error("expected a freshly recorded worktree to be active")
	}
	if got.Branch != "feature/x" {
		t.Errorf("expected Branch=feature/x, got %q", got.Branch)
	}
}

func TestRecordWorktreeCreated_RejectsEmptyPath(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RecordWorktreeCreatedInput{ProjectID: "p1", RepoID: "r1", Path: "", Branch: "main"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID")
}

func TestRecordWorktreeCreated_RequiresTenantContext(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewRecordWorktreeCreated(repo)

	_, err := uc.Execute(context.Background(), RecordWorktreeCreatedInput{ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
