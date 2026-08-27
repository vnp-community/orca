package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RemoveWorktree has no compensating action on bookkeeping failure — see
// this package's ports.go doc comment for why "on-disk gone, bookkeeping
// stale" is a safe terminal state, unlike CreateWorktree's failure
// direction: "worktree doesn't exist" is a safe terminal state to leave a
// stale bookkeeping record pointing at (unlike a create failure, which
// leaves live, unaccounted-for disk usage and a dangling branch).
//
// worktreeID is passed straight into dispatchExecutor here (not through
// ProjectClient.GetRepo first) — RemoveWorktreeRequest carries a
// worktree_id directly, which IS the dispatch key ConnectionResolver
// expects (see resolver.go's ResolveConnection doc comment), unlike the
// repo-scoped usecases (CreateWorktree, DetectWorktrees, ...) that only
// have a repo_id and must resolve it via GetRepo first.
type RemoveWorktree struct {
	resolver ConnectionResolver
	projects ProjectClient
	scm      SCMClient
	local    GitExecutor
	relay    GitExecutor
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, scm SCMClient, local, relay GitExecutor) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, scm: scm, local: local, relay: relay}
}

// Execute enforces both of BR-AT-11 (uncommitted changes) and BR-AT-12
// (open PR) before the actual `git worktree remove` — unenforced before
// this task, for every caller (manual worktree.rm AND the automated bulk
// cleanup_worktrees step, which always calls with force=false,
// allowOpenPR=false, per workflow-service.CleanupWorktreesStepExecutor's
// doc comment: an automated bulk delete must never bypass either rule).
func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force, allowOpenPR bool) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-AT-11 — fails closed on its own check error: an uncommitted-changes
	// check that can't run must not silently let a dirty worktree through.
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_CHECK_FAILED", "failed to check worktree status before removal", err)
	}
	if len(status.Files) > 0 && !force {
		return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES", "worktree has uncommitted changes", nil)
	}

	// BR-AT-12 — independent of force; requires the SEPARATE allow_open_pr
	// override. A GetPullRequestForBranch error (no SCM integration
	// configured, no tenant in context, or the KNOWN GAP in
	// internal/adapter/scmclient) fails OPEN on this check only — a repo
	// with no way to answer "does this branch have an open PR" must not
	// become permanently undeletable.
	if branch := status.Branch; branch != "" {
		if tenantID, ok := tenant.TenantID(ctx); ok {
			pr, found, err := uc.scm.GetPullRequestForBranch(ctx, tenantID, branch)
			if err == nil && found && pr.State == "open" && !allowOpenPR {
				return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_OPEN_PR", "worktree's branch has an open pull request", nil)
			}
		}
	}

	if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return nil
}
