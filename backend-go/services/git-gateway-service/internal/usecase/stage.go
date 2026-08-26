package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type StageInput struct {
	WorktreeID string
	Paths      []string
}

// Stage resolves the worktree's owning host and stages the given paths
// there. Backs both git.stage (single path) and git.bulkStage (multiple) —
// see RelayExecutor.Stage's doc comment for why one usecase covers both.
type Stage struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewStage(resolver ConnectionResolver, local, relay GitExecutor) *Stage {
	return &Stage{resolver: resolver, local: local, relay: relay}
}

func (uc *Stage) Execute(ctx context.Context, in StageInput) (domain.SimpleResult, error) {
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
	result, err := executor.Stage(ctx, repoPath, in.Paths)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_STAGE_FAILED", "failed to stage paths", err)
	}
	return result, nil
}
