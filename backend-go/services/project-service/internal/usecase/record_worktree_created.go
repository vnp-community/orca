package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RecordWorktreeCreatedInput struct {
	ProjectID      string
	RepoID         string
	Path           string
	Branch         string
	IdempotencyKey string
}

// RecordWorktreeCreated is called by git-gateway-service AFTER the real
// `git worktree add` succeeded on the Dev Server Agent — this usecase only
// writes the bookkeeping row, it never triggers the git operation itself.
// See domain.Worktree's doc comment.
type RecordWorktreeCreated struct {
	repo WorktreeRepository
}

func NewRecordWorktreeCreated(repo WorktreeRepository) *RecordWorktreeCreated {
	return &RecordWorktreeCreated{repo: repo}
}

func (uc *RecordWorktreeCreated) Execute(ctx context.Context, in RecordWorktreeCreatedInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	wt, err := domain.NewWorktree(uuid.NewString(), in.ProjectID, in.RepoID, in.Path, in.Branch, in.IdempotencyKey)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID", err.Error(), err)
	}

	created, err := uc.repo.RecordWorktreeCreated(ctx, wt)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_RECORD_WORKTREE_FAILED", "failed to persist worktree", err)
	}
	return created, nil
}
