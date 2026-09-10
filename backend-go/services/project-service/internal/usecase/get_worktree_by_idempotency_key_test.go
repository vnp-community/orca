package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestGetWorktreeByIdempotencyKey_FoundReturnsWorktree(t *testing.T) {
	repo := newFakeWorktreeRepository()
	key := "sha256-abc"
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main", Active: true, IdempotencyKey: &key}
	uc := NewGetWorktreeByIdempotencyKey(repo)

	got, found, err := uc.Execute(context.Background(), "p1", key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if got.ID != "w1" {
		t.Errorf("expected ID=w1, got %q", got.ID)
	}
}

func TestGetWorktreeByIdempotencyKey_NoMatchReturnsFoundFalse(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewGetWorktreeByIdempotencyKey(repo)

	_, found, err := uc.Execute(context.Background(), "p1", "no-such-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false")
	}
}
