package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ListLocalBranchesInput struct {
	WorktreeID string
}

type ListLocalBranches struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewListLocalBranches(resolver ConnectionResolver, local, relay GitExecutor) *ListLocalBranches {
	return &ListLocalBranches{resolver: resolver, local: local, relay: relay}
}

func (uc *ListLocalBranches) Execute(ctx context.Context, in ListLocalBranchesInput) ([]domain.BranchInfo, error) {
	if in.WorktreeID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	branches, err := executor.ListLocalBranches(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_LIST_BRANCHES_FAILED", "failed to list local branches", err)
	}
	return branches, nil
}
