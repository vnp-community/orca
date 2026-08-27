package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// GetWorktreeByIdempotencyKey backs BR-CLI-01 — git-gateway-service's
// CreateWorktree saga calls this before running `git worktree add`.
// found=false means "no dedupe match yet", not an error.
type GetWorktreeByIdempotencyKey struct {
	repo WorktreeRepository
}

func NewGetWorktreeByIdempotencyKey(repo WorktreeRepository) *GetWorktreeByIdempotencyKey {
	return &GetWorktreeByIdempotencyKey{repo: repo}
}

func (uc *GetWorktreeByIdempotencyKey) Execute(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error) {
	return uc.repo.FindWorktreeByIdempotencyKey(ctx, projectID, idempotencyKey)
}
