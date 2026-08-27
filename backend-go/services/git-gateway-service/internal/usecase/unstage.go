package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type UnstageInput struct {
	WorktreeID string
	Paths      []string
}

// Unstage resolves the worktree's owning host and unstages the given paths
// there. Backs both git.unstage (single path) and git.bulkUnstage
// (multiple) — see RelayExecutor.Unstage's doc comment.
type Unstage struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewUnstage(resolver ConnectionResolver, local, relay GitExecutor) *Unstage {
	return &Unstage{resolver: resolver, local: local, relay: relay}
}

func (uc *Unstage) Execute(ctx context.Context, in UnstageInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if len(in.Paths) == 0 {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATHS", "paths is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Unstage(ctx, repoPath, in.Paths)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_UNSTAGE_FAILED", "failed to unstage paths", err)
	}
	return result, nil
}
