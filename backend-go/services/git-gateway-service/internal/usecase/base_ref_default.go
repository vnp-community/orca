package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type BaseRefDefaultInput struct {
	WorktreeID string
}

// BaseRefDefault resolves the worktree's owning host and returns its
// default base ref — same resolve -> dispatch -> translate flow as
// GetStatus (get_status.go).
type BaseRefDefault struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewBaseRefDefault(resolver ConnectionResolver, local, relay GitExecutor) *BaseRefDefault {
	return &BaseRefDefault{resolver: resolver, local: local, relay: relay}
}

func (uc *BaseRefDefault) Execute(ctx context.Context, in BaseRefDefaultInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	ref, err := executor.BaseRefDefault(ctx, repoPath)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_BASE_REF_DEFAULT_FAILED", "failed to resolve default base ref", err)
	}
	return ref, nil
}
