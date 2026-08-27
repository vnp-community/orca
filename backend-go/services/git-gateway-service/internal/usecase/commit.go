package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CommitInput struct {
	WorktreeID string
	Message    string
	Paths      []string // empty = all staged
}

// Commit resolves the worktree's owning host and creates a commit there.
type Commit struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCommit(resolver ConnectionResolver, local, relay GitExecutor) *Commit {
	return &Commit{resolver: resolver, local: local, relay: relay}
}

func (uc *Commit) Execute(ctx context.Context, in CommitInput) (domain.CommitResult, error) {
	if in.WorktreeID == "" {
		return domain.CommitResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.Message == "" {
		return domain.CommitResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_MESSAGE", "commit message is required", nil)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.CommitResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}

	result, err := executor.Commit(ctx, repoPath, in.Message, in.Paths)
	if err != nil {
		return domain.CommitResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_COMMIT_FAILED", "failed to commit", err)
	}
	return result, nil
}
