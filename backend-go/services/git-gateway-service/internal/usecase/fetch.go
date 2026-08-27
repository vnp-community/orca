package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// FetchInput's PushTarget is optional (nil = let the executor resolve the
// worktree's configured push target/remote) — the real git.fetch always
// prunes and has no separate `remote`/`prune` fields, see
// GitExecutor.Fetch's doc comment.
type FetchInput struct {
	WorktreeID string
	PushTarget *domain.PushTargetInput
}

type Fetch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewFetch(resolver ConnectionResolver, local, relay GitExecutor) *Fetch {
	return &Fetch{resolver: resolver, local: local, relay: relay}
}

func (uc *Fetch) Execute(ctx context.Context, in FetchInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Fetch(ctx, repoPath, in.PushTarget)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_FETCH_FAILED", "failed to fetch", err)
	}
	return result, nil
}
