package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type UpstreamStatusInput struct {
	WorktreeID string
	// PushTarget is a placeholder string field — see SOL-032 §0 open
	// question #1. Optional.
	PushTarget string
}

// UpstreamStatus is the one method in TASK-209's history/compare group
// that still needs TASK-227 (agent reachability) — see this usecase's
// task file for details. Code-complete regardless.
type UpstreamStatus struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewUpstreamStatus(resolver ConnectionResolver, local, relay GitExecutor) *UpstreamStatus {
	return &UpstreamStatus{resolver: resolver, local: local, relay: relay}
}

func (uc *UpstreamStatus) Execute(ctx context.Context, in UpstreamStatusInput) (domain.UpstreamStatus, error) {
	if in.WorktreeID == "" {
		return domain.UpstreamStatus{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.UpstreamStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.UpstreamStatus(ctx, repoPath, in.PushTarget)
	if err != nil {
		return domain.UpstreamStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_UPSTREAM_STATUS_FAILED", "failed to compute upstream status", err)
	}
	return result, nil
}
