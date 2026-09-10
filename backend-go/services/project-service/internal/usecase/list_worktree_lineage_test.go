package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestListWorktreeLineage_ReturnsOnlyWorktreesWithCapturedParent(t *testing.T) {
	repo := newFakeWorktreeRepository()
	parent := "w1"
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main"}
	repo.worktrees["w2"] = domain.Worktree{ID: "w2", ProjectID: "p1", RepoID: "r1", Path: "/srv/w2", Branch: "feature", ParentWorktreeID: &parent}
	uc := NewListWorktreeLineage(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	got, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "w2" {
		t.Fatalf("expected only w2 (has a captured parent), got %+v", got)
	}
}

func TestListWorktreeLineage_RequiresTenant(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewListWorktreeLineage(repo)

	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error with no tenant in context")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindUnauthenticated {
		t.Fatalf("expected KindUnauthenticated, got %v", err)
	}
}

func TestListWorktreeLineage_WrapsRepositoryError(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.listLineageErr = errors.New("boom")
	uc := NewListWorktreeLineage(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("expected an error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindInternal {
		t.Fatalf("expected KindInternal, got %v", err)
	}
}
