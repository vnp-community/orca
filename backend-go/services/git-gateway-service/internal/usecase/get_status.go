package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// GetStatusInput mirrors the gRPC request 1:1, per this repo's convention
// that usecase granularity mirrors today's RPC methods.
type GetStatusInput struct {
	WorktreeID string
}

// GetStatus resolves the worktree's owning host and returns its git status —
// the reference resolve -> dispatch -> translate flow, §2/§7.
type GetStatus struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewGetStatus(resolver ConnectionResolver, local, relay GitExecutor) *GetStatus {
	return &GetStatus{resolver: resolver, local: local, relay: relay}
}

func (uc *GetStatus) Execute(ctx context.Context, in GetStatusInput) (domain.GitStatus, error) {
	if in.WorktreeID == "" {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}

	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_STATUS_FAILED", "failed to get git status", err)
	}
	return status, nil
}
