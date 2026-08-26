package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type DiscardInput struct {
	WorktreeID string
	Path       string
}

type Discard struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewDiscard(resolver ConnectionResolver, local, relay GitExecutor) *Discard {
	return &Discard{resolver: resolver, local: local, relay: relay}
}

func (uc *Discard) Execute(ctx context.Context, in DiscardInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.Path == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATH", "path is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Discard(ctx, repoPath, in.Path)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_DISCARD_FAILED", "failed to discard path", err)
	}
	return result, nil
}
