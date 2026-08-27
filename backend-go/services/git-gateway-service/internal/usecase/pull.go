package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type PullInput struct {
	WorktreeID string
}

// Pull resolves the worktree's owning host and pulls there.
type Pull struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewPull(resolver ConnectionResolver, local, relay GitExecutor) *Pull {
	return &Pull{resolver: resolver, local: local, relay: relay}
}

func (uc *Pull) Execute(ctx context.Context, in PullInput) (domain.PullResult, error) {
	if in.WorktreeID == "" {
		return domain.PullResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.PullResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}

	result, err := executor.Pull(ctx, repoPath)
	if err != nil {
		return domain.PullResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_PULL_FAILED", "failed to pull", err)
	}
	return result, nil
}
