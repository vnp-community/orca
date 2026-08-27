package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// SubmoduleStatusInput's SubmodulePath is required — the real
// git.submoduleStatus operates on ONE submodule per call, not "list every
// submodule" (SOL-032 §0 open question #3, resolved — see
// GitExecutor.SubmoduleStatus's doc comment for the frontend-caller
// citation that closes this question). Area defaults to "unstaged" when
// empty, matching the real agent's own default.
type SubmoduleStatusInput struct {
	WorktreeID    string
	SubmodulePath string
	Area          string
}

type SubmoduleStatus struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewSubmoduleStatus(resolver ConnectionResolver, local, relay GitExecutor) *SubmoduleStatus {
	return &SubmoduleStatus{resolver: resolver, local: local, relay: relay}
}

func (uc *SubmoduleStatus) Execute(ctx context.Context, in SubmoduleStatusInput) (domain.GitStatus, error) {
	if in.WorktreeID == "" {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.SubmodulePath == "" {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_SUBMODULE_PATH", "submodule_path is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.SubmoduleStatus(ctx, repoPath, in.SubmodulePath, in.Area)
	if err != nil {
		return domain.GitStatus{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SUBMODULE_STATUS_FAILED", "failed to get submodule status", err)
	}
	return result, nil
}
