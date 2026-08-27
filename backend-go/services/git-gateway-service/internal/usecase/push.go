package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type PushInput struct {
	WorktreeID string
	Remote     string
	Branch     string
}

// Push resolves the worktree's owning host and pushes there.
type Push struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewPush(resolver ConnectionResolver, local, relay GitExecutor) *Push {
	return &Push{resolver: resolver, local: local, relay: relay}
}

func (uc *Push) Execute(ctx context.Context, in PushInput) (domain.PushResult, error) {
	if in.WorktreeID == "" {
		return domain.PushResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.PushResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}

	result, err := executor.Push(ctx, repoPath, in.Remote, in.Branch)
	if err != nil {
		return domain.PushResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_PUSH_FAILED", "failed to push", err)
	}
	return result, nil
}
