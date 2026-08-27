package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ForkSyncInput struct {
	WorktreeID       string
	ExpectedUpstream string
}

// ForkSync requires ExpectedUpstream — the real agent rejects calls
// without it, see TASK-209's Contract correction section.
type ForkSync struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewForkSync(resolver ConnectionResolver, local, relay GitExecutor) *ForkSync {
	return &ForkSync{resolver: resolver, local: local, relay: relay}
}

func (uc *ForkSync) Execute(ctx context.Context, in ForkSyncInput) (domain.ForkSyncStatus, error) {
	if in.WorktreeID == "" {
		return domain.ForkSyncStatus{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.ExpectedUpstream == "" {
		return domain.ForkSyncStatus{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_EXPECTED_UPSTREAM", "expected_upstream is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.ForkSyncStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.ForkSync(ctx, repoPath, in.ExpectedUpstream)
	if err != nil {
		return domain.ForkSyncStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_FORK_SYNC_FAILED", "failed to compute fork sync status", err)
	}
	return result, nil
}
