package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type CheckIgnoredInput struct {
	WorktreeID string
	Paths      []string
}

// CheckIgnored returns []string (the ignored subset), not a
// domain.IgnoredPath-per-input-path pair — matches the real agent's
// response shape, see TASK-209's Contract correction section.
type CheckIgnored struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCheckIgnored(resolver ConnectionResolver, local, relay GitExecutor) *CheckIgnored {
	return &CheckIgnored{resolver: resolver, local: local, relay: relay}
}

func (uc *CheckIgnored) Execute(ctx context.Context, in CheckIgnoredInput) ([]string, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if len(in.Paths) == 0 {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATHS", "paths is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	ignored, err := executor.CheckIgnored(ctx, repoPath, in.Paths)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CHECK_IGNORED_FAILED", "failed to check ignored paths", err)
	}
	return ignored, nil
}
