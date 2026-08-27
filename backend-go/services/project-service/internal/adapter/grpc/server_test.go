package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
	"github.com/stablyai/orca-go/services/project-service/internal/usecase"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeWorktreeRepository is a minimal usecase.WorktreeRepository double for
// exercising GetWorktree's wire<->usecase translation (SOL-WT-04) — this
// package had no test file before this contract test.
type fakeWorktreeRepository struct {
	worktrees map[string]domain.Worktree
}

func (f *fakeWorktreeRepository) RecordWorktreeCreated(context.Context, domain.Worktree, domain.OutboxEvent) (domain.Worktree, error) {
	return domain.Worktree{}, nil
}
func (f *fakeWorktreeRepository) RecordWorktreeRemoved(context.Context, string) error { return nil }
func (f *fakeWorktreeRepository) ListWorktrees(context.Context, string) ([]domain.Worktree, error) {
	return nil, nil
}

func (f *fakeWorktreeRepository) GetWorktree(_ context.Context, worktreeID string) (domain.Worktree, error) {
	wt, ok := f.worktrees[worktreeID]
	if !ok {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	return wt, nil
}

func (f *fakeWorktreeRepository) SetWorktreeActivation(context.Context, string, bool) (domain.Worktree, error) {
	return domain.Worktree{}, nil
}
func (f *fakeWorktreeRepository) RenameWorktree(context.Context, string, string) (domain.Worktree, error) {
	return domain.Worktree{}, nil
}

func TestServer_GetWorktree_TranslatesResult(t *testing.T) {
	baseRef := "develop"
	repo := &fakeWorktreeRepository{worktrees: map[string]domain.Worktree{
		"wt-1": {ID: "wt-1", ProjectID: "p1", RepoID: "r1", Path: "/srv/w1", Branch: "feature", Active: true, BaseRef: &baseRef},
	}}
	s := &Server{getWorktree: usecase.NewGetWorktree(repo)}

	resp, err := s.GetWorktree(context.Background(), &projectv1.GetWorktreeRequest{WorktreeId: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetId() != "wt-1" || resp.GetPath() != "/srv/w1" || resp.GetBranch() != "feature" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.GetBaseRef() != baseRef {
		t.Errorf("expected base_ref=%q, got %q", baseRef, resp.GetBaseRef())
	}
}

func TestServer_GetWorktree_NotFound_ReturnsNotFound(t *testing.T) {
	repo := &fakeWorktreeRepository{worktrees: map[string]domain.Worktree{}}
	s := &Server{getWorktree: usecase.NewGetWorktree(repo)}

	_, err := s.GetWorktree(context.Background(), &projectv1.GetWorktreeRequest{WorktreeId: "unknown"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}
