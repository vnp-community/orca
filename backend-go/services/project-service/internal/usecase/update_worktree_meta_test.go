package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestUpdateWorktreeMeta_MergesPatch(t *testing.T) {
	repo := newFakeWorktreeRepository()
	repo.worktrees["w1"] = domain.Worktree{ID: "w1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "main", Active: true, Metadata: json.RawMessage(`{"displayName":"old"}`)}
	uc := NewUpdateWorktreeMeta(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, UpdateWorktreeMetaInput{WorktreeID: "w1", Metadata: json.RawMessage(`{"isPinned":true}`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(got.Metadata, &m); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if m["displayName"] != "old" || m["isPinned"] != true {
		t.Errorf("expected merged metadata, got %+v", m)
	}
}

func TestUpdateWorktreeMeta_RequiresWorktreeID(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewUpdateWorktreeMeta(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateWorktreeMetaInput{Metadata: json.RawMessage(`{}`)})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED")
}

func TestUpdateWorktreeMeta_NotFound(t *testing.T) {
	repo := newFakeWorktreeRepository()
	uc := NewUpdateWorktreeMeta(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateWorktreeMetaInput{WorktreeID: "missing", Metadata: json.RawMessage(`{}`)})
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND")
}
