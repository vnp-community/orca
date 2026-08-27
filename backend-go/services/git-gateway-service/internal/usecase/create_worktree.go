package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef, IdempotencyKey string
}

// CreateWorktree is the saga: resolve host, run `git worktree add`, then
// record bookkeeping via project-service. If bookkeeping fails AFTER the
// git operation succeeded, best-effort compensate by removing the
// just-created worktree — see this package's ports.go doc comment and
// SOL-031 for the full rationale.
//
// Source of truth, stated explicitly: git-gateway-service (via the Dev
// Server Agent or local exec) is authoritative for on-disk existence;
// project-service is authoritative for bookkeeping metadata. Compensation
// is best-effort, not guaranteed — a crash between the agent's `git
// worktree add` succeeding and the compensating `git worktree remove`
// running leaves a genuine orphan; DetectWorktrees/worktree.detectedList
// is the reconciliation safety net for exactly that failure window, not
// optional polish.
type CreateWorktree struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewCreateWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *CreateWorktree {
	return &CreateWorktree{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
	if in.IdempotencyKey != "" {
		if existing, found, err := uc.projects.FindWorktreeByIdempotencyKey(ctx, in.ProjectID, in.IdempotencyKey); err != nil {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_IDEMPOTENCY_LOOKUP_FAILED", "failed to check for existing worktree", err)
		} else if found {
			// BR-CLI-01: same (project_id, idempotency_key) -> return the
			// existing worktree, not a second `git worktree add` attempt.
			// HeadSHA is not stored on the bookkeeping record (project-service's
			// Worktree message has no head_sha field) — left empty here rather
			// than re-resolving it with an extra GetStatus call the caller
			// didn't ask for; orca-cli's Result.HeadSHA is therefore only
			// populated on a genuinely fresh create.
			return domain.WorktreeResult{WorktreeID: existing.ID, Path: existing.Path}, nil
		}
	}

	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}

	// dispatchExecutor's key is the repo confirmed by GetRepo (repo.ID),
	// not the raw request field — see ports.go's dispatchExecutor doc
	// comment for why that distinction matters here.
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	result, err := executor.CreateWorktree(ctx, repoPath, in.Branch, in.BaseRef)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "git worktree add failed", err)
	}

	worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch)
	if err != nil {
		// Compensating step (05-data-architecture.md's saga pattern) — the
		// git op already succeeded; project-service has no record of it.
		if compErr := executor.RemoveWorktree(ctx, result.Path, true); compErr != nil {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED",
				fmt.Sprintf("worktree created but bookkeeping failed (%v) and rollback also failed (%v) — orphaned at %s, will surface via worktree.detectedList", err, compErr, result.Path), err)
		}
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED", "worktree created but bookkeeping failed; rolled back cleanly", err)
	}
	return domain.WorktreeResult{WorktreeID: worktree.ID, Path: result.Path, HeadSHA: result.HeadSHA}, nil
}
