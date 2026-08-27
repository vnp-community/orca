package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// Cursor field dropped, Ref renamed to BaseRef — the real agent has no
// pagination concept (see TASK-209's Contract correction section).
type HistoryInput struct {
	WorktreeID string
	BaseRef    string
	Limit      int
}

type HistoryResult struct {
	Commits []domain.CommitRef
}

type History struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewHistory(resolver ConnectionResolver, local, relay GitExecutor) *History {
	return &History{resolver: resolver, local: local, relay: relay}
}

func (uc *History) Execute(ctx context.Context, in HistoryInput) (HistoryResult, error) {
	if in.WorktreeID == "" {
		return HistoryResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return HistoryResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	commits, err := executor.History(ctx, repoPath, in.BaseRef, in.Limit)
	if err != nil {
		return HistoryResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_HISTORY_FAILED", "failed to fetch history", err)
	}
	return HistoryResult{Commits: commits}, nil
}
