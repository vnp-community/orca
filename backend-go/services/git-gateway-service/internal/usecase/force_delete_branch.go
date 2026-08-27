package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ForceDeleteBranch is the direct regression test target for BUG-031's old
// TS crash-bug class: forceDeletePreservedBranch? was optional and only
// one provider implemented it. Go's compiler now refuses to build ANY
// GitExecutor implementation missing this method (ports.go), closing that
// gap by construction, not by convention.
type ForceDeleteBranch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewForceDeleteBranch(resolver ConnectionResolver, local, relay GitExecutor) *ForceDeleteBranch {
	return &ForceDeleteBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *ForceDeleteBranch) Execute(ctx context.Context, worktreeID, branch string) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	if err := executor.ForceDeleteBranch(ctx, repoPath, branch); err != nil {
		// domain.ErrForceDeleteBranchUnsupported, not a grpcclient-local
		// sentinel — see its doc comment for the import-cycle this avoids
		// (grpcclient already imports usecase for the port interfaces it
		// implements, so usecase importing grpcclient back would cycle).
		if errors.Is(err, domain.ErrForceDeleteBranchUnsupported) {
			return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_FORCE_DELETE_UNSUPPORTED", "the target dev server's agent does not support force-deleting a branch", err)
		}
		return apperrors.New(apperrors.KindInternal, "WORKTREE_FORCE_DELETE_FAILED", "failed to force-delete branch", err)
	}
	return nil
}
