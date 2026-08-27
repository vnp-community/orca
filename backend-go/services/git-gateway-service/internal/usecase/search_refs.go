package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type SearchRefsInput struct {
	WorktreeID string
	Query      string
}

// SearchRefs follows GetStatus's exact resolve -> dispatch -> translate shape.
type SearchRefs struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewSearchRefs(resolver ConnectionResolver, local, relay GitExecutor) *SearchRefs {
	return &SearchRefs{resolver: resolver, local: local, relay: relay}
}

func (uc *SearchRefs) Execute(ctx context.Context, in SearchRefsInput) ([]string, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	refs, err := executor.SearchRefs(ctx, repoPath, in.Query)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SEARCH_REFS_FAILED", "failed to search refs", err)
	}
	return refs, nil
}
