package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ResolveConflictInput is the per-file ours/theirs/markResolved operation —
// see ResolveConflictRequest's proto doc comment for why this has no real
// agent RPC backing it on the relay side.
type ResolveConflictInput struct {
	WorktreeID string
	Path       string
	Operation  string
}

type ResolveConflict struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewResolveConflict(resolver ConnectionResolver, local, relay GitExecutor) *ResolveConflict {
	return &ResolveConflict{resolver: resolver, local: local, relay: relay}
}

func (uc *ResolveConflict) Execute(ctx context.Context, in ResolveConflictInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.Path == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATH", "path is required", nil)
	}
	if in.Operation == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_OPERATION", "operation is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.ResolveConflict(ctx, repoPath, in.Path, in.Operation)
	if err != nil {
		// domain.ErrConflictResolveUnsupportedOverRelay — same
		// operational-fallback pattern as ForceDeleteBranch's
		// domain.ErrForceDeleteBranchUnsupported check.
		if errors.Is(err, domain.ErrConflictResolveUnsupportedOverRelay) {
			return domain.SimpleResult{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_CONFLICT_RESOLVE_UNSUPPORTED", "the target dev server's agent does not support per-file conflict resolution", err)
		}
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CONFLICT_RESOLVE_FAILED", "failed to resolve conflict", err)
	}
	return result, nil
}
