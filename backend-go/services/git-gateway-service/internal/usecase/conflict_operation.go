package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ConflictOperationInput requests the operation-in-progress detector — see
// GitExecutor.ConflictOperation's doc comment for why this has no
// path/operation fields (the original TASK-207 sketch conflated the
// detector with the per-file resolver; see ResolveConflict for that).
type ConflictOperationInput struct {
	WorktreeID string
}

type ConflictOperation struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewConflictOperation(resolver ConnectionResolver, local, relay GitExecutor) *ConflictOperation {
	return &ConflictOperation{resolver: resolver, local: local, relay: relay}
}

// Execute returns the operation-in-progress ("merge"/"rebase"/
// "cherry-pick"/"unknown") for the worktree.
func (uc *ConflictOperation) Execute(ctx context.Context, in ConflictOperationInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	operation, err := executor.ConflictOperation(ctx, repoPath)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_CONFLICT_OPERATION_FAILED", "failed to detect conflict operation", err)
	}
	return operation, nil
}
