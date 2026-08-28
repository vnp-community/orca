package usecase

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// CompareWorktrees belongs in git-gateway-service's own usecase layer, not
// api-gateway's edge — unlike worktree.detectedList's genuine two-service
// merge, this only needs data git-gateway-service already computes
// (BranchCompare per worktree) plus one small project-service metadata
// lookup, matching the shape CreateWorktree itself already has (GetRepo
// call, then dispatch). This is a fan-in read — fail-fast via
// errgroup.WithContext is correct here (unlike SOL-WT-02's fan-out, a
// partial comparison is not useful).
type CompareWorktrees struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewCompareWorktrees(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *CompareWorktrees {
	return &CompareWorktrees{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *CompareWorktrees) Execute(ctx context.Context, worktreeIDs []string) (domain.CompareWorktreesResult, error) {
	if len(worktreeIDs) < 2 {
		return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindInvalidArgument, "COMPARE_NEEDS_AT_LEAST_TWO", "at least 2 worktrees required", nil)
	}

	// BR-WT-13 — every worktree must share the same base_ref.
	var sharedBaseRef string
	metas := make([]domain.WorktreeInfo, len(worktreeIDs))
	for i, id := range worktreeIDs {
		wt, err := uc.projects.GetWorktree(ctx, id)
		if err != nil {
			return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_NOT_FOUND", "worktree not found", err)
		}
		if wt.BaseRef == "" {
			return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_BASE_REF_UNKNOWN",
				"this worktree has no recorded base_ref (created before the base_ref backfill) — cannot validate BR-WT-13", nil)
		}
		if i == 0 {
			sharedBaseRef = wt.BaseRef
		} else if wt.BaseRef != sharedBaseRef {
			return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_COMPARE_BASE_MISMATCH",
				fmt.Sprintf("worktree %s has base_ref %q, expected %q", id, wt.BaseRef, sharedBaseRef), nil)
		}
		metas[i] = wt
	}

	// Fan-in read: fail-fast IS correct here (unlike SOL-WT-02's fan-out) —
	// this is a pure read aggregation, a partial comparison is not useful.
	results := make([]domain.WorktreeComparison, len(worktreeIDs))
	g, gctx := errgroup.WithContext(ctx)
	for i, wt := range metas {
		g.Go(func() error {
			executor, repoPath, err := dispatchExecutor(gctx, uc.resolver, uc.local, uc.relay, wt.ID)
			if err != nil {
				return err
			}
			cmp, err := executor.BranchCompare(gctx, repoPath, sharedBaseRef)
			if err != nil {
				return err
			}
			addedLines, removedLines := 0, 0
			for _, e := range cmp.Entries {
				addedLines += e.Added
				removedLines += e.Removed
			}
			results[i] = domain.WorktreeComparison{
				WorktreeID: wt.ID, ChangedFiles: cmp.ChangedFiles,
				AddedLines: addedLines, RemovedLines: removedLines,
				MergeBase: cmp.MergeBase, Status: cmp.Status, ErrorMessage: cmp.ErrorMessage,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return domain.CompareWorktreesResult{}, apperrors.New(apperrors.KindInternal, "COMPARE_WORKTREES_FAILED", "failed to compare one or more worktrees", err)
	}

	// BR-WT-14 (soft check): a differing merge_base across entries means at
	// least one worktree has a stale/unfetched view of sharedBaseRef —
	// surfaced via each WorktreeComparison.MergeBase for the UI to diff
	// itself, not enforced as a hard failure here (the backend must not
	// silently run an implicit fetch the user didn't ask for).
	//
	// BR-WT-15 (no auto-selected winner) needs no enforcement code —
	// CompareWorktreesResult carries comparison data only, with no
	// ranking/scoring/winner field; keep it that way as a deliberate
	// response-shape invariant, not an oversight.
	return domain.CompareWorktreesResult{BaseRef: sharedBaseRef, Worktrees: results}, nil
}
