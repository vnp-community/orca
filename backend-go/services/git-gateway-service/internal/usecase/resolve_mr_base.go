package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ResolveMrBase — identical shape to ResolvePrBase, via
// SCMClient.GetMergeRequestBase instead. See detect_worktrees.go's doc
// comment for why this resolves repoID via ProjectClient.GetRepo before
// dispatching, rather than passing repoID straight into dispatchExecutor.
type ResolveMrBase struct {
	scm      SCMClient
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewResolveMrBase(scm SCMClient, resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *ResolveMrBase {
	return &ResolveMrBase{scm: scm, resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *ResolveMrBase) Execute(ctx context.Context, repoID string, mrNumber int32) (domain.ResolvedBase, error) {
	baseBranch, _, err := uc.scm.GetMergeRequestBase(ctx, repoID, mrNumber)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_MR_BASE_LOOKUP_FAILED", "failed to resolve MR base", err)
	}
	repo, err := uc.projects.GetRepo(ctx, repoID)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	resolvedSHA, err := executor.FetchAndResolveRef(ctx, repoPath, baseBranch)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BASE_REF_UNRESOLVABLE", "MR base branch not resolvable in local repo", err)
	}
	return domain.ResolvedBase{Branch: baseBranch, SHA: resolvedSHA}, nil
}
