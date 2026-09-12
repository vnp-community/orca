package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef, IdempotencyKey, Name, Path string
	// Lineage is empty for a plain CreateWorktree call — only
	// CreateWorktreeFromIssue (SOL-PI-02) populates it, threading the
	// linked-issue reference through to project-service in the same saga
	// step rather than duplicating RecordWorktreeCreated logic.
	Lineage domain.WorktreeLineageCapture
}

// CreateWorktree is the saga: validate (BR-WT-01/04, [A1]/[A2]/[A3] per
// SOL-WT-01), resolve host, run `git worktree add`, then record bookkeeping
// via project-service. If bookkeeping fails AFTER the git operation
// succeeded, best-effort compensate by removing the just-created worktree —
// see this package's ports.go doc comment and SOL-031 for the full
// rationale.
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
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewCreateWorktree(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *CreateWorktree {
	return &CreateWorktree{reachability: reachability, projects: projects, local: local, relay: relay}
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

	name := in.Name
	if name == "" {
		name = sanitizeBranchForPathUsecase(in.Branch)
	}
	if err := domain.ValidateWorktreeName(name); err != nil { // BR-WT-01
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_NAME_INVALID", err.Error(), err)
	}

	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}

	// dispatchExecutorForRepo's key is the repo confirmed by GetRepo, not
	// the raw request field — see ports.go's doc comment for why this
	// (not dispatchExecutor/ConnectionResolver) is the correct dispatch
	// for a repo-scoped usecase.
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-WT-04 — count active worktrees for this repo before attempting
	// git. Fails OPEN on a ListWorktrees error: a transient bookkeeping
	// read failure must not block worktree creation.
	if existing, err := uc.projects.ListWorktrees(ctx, in.ProjectID); err == nil {
		count := 0
		for _, w := range existing {
			if w.RepoID == in.RepoID && w.Active {
				count++
			}
		}
		if count >= 20 {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_LIMIT_EXCEEDED", "maximum 20 worktrees per repository", nil)
		}
	}

	// [A1] — duplicate-path pre-check + alternate-name suggestion via the
	// already-required ListWorktreePaths; best-effort, git itself is still
	// the final authority if this call fails.
	// ListWorktreePaths now returns domain.WorktreeGitInfo (path + HEAD sha +
	// branch), not bare paths (see ports.go's GitExecutor.ListWorktreePaths
	// doc comment — DetectWorktrees/worktree.detectedList need the git-level
	// identity too) — only the path is needed for this pre-check.
	onDisk, _ := executor.ListWorktreePaths(ctx, repoPath)
	taken := make(map[string]bool, len(onDisk))
	for _, p := range onDisk {
		taken[p.Path] = true
	}
	targetPath := in.Path
	if targetPath == "" {
		targetPath = repoPath + "-" + name
	}
	if taken[targetPath] {
		suggested := domain.SuggestAlternateName(name, taken)
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindAlreadyExists, "WORKTREE_PATH_EXISTS",
			fmt.Sprintf("path already exists; try %q", suggested), nil)
	}

	// [A1b] — branch-already-checked-out-elsewhere pre-check (incident
	// 2026-09-12): git refuses `worktree add` for a branch that's already
	// the HEAD of another worktree (including the main repo clone itself,
	// per git-worktree(1)) with "fatal: '<branch>' is already used by
	// worktree at '<path>'" — a real, common case for a branch the caller
	// picked from an "existing branches" list (e.g. "main", almost always
	// already checked out in repoPath itself) rather than a genuinely new
	// branch name. Surfacing this clearly here (using the same onDisk data
	// as [A1] above) avoids a confusing generic WORKTREE_CREATE_FAILED
	// round-trip through the agent for a case we can already detect locally.
	for _, w := range onDisk {
		if w.Branch == "refs/heads/"+in.Branch {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_BRANCH_CHECKED_OUT_ELSEWHERE",
				fmt.Sprintf("branch %q is already checked out at %q; pick a different branch or open that existing worktree instead", in.Branch, w.Path), nil)
		}
	}

	result, err := executor.CreateWorktree(ctx, repoPath, in.Branch, in.BaseRef, targetPath)
	if err != nil {
		if isBaseRefNotFoundErr(err) { // [A2]
			if branches, listErr := executor.ListLocalBranches(ctx, repoPath); listErr == nil {
				names := make([]string, 0, len(branches))
				for _, b := range branches {
					names = append(names, b.Name)
				}
				return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_BASE_REF_NOT_FOUND",
					fmt.Sprintf("branch %q not found; available: %s", in.BaseRef, strings.Join(names, ", ")), err)
			}
		}
		// Include err's own text in Message (not just Err, which
		// apperrors.ToGRPCStatus never sends over the wire) — this usecase's
		// failures are internal validation/git-state errors, not sensitive,
		// and the generic "git worktree add failed" alone left every past
		// live failure needing a server-side log dive to diagnose.
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", fmt.Sprintf("git worktree add failed: %s", err), err)
	}

	// repo.ProjectID (resolved server-side via GetRepo above), not the wire's
	// in.ProjectID: no real caller ever sends project_id on this RPC (the
	// frontend's worktree.create only ever sends repo/name/baseBranch), so
	// trusting it left bookkeeping recorded under an empty project id.
	worktree, err := uc.projects.RecordWorktreeCreated(ctx, repo.ProjectID, in.RepoID, result.Path, in.Branch, in.BaseRef, in.Lineage)
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

// isBaseRefNotFoundErr classifies git's stderr — same pragmatic string-match
// approach this package already uses elsewhere (e.g. localgit's
// strings.HasPrefix(baseRef, "-") flag-injection guard).
func isBaseRefNotFoundErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "invalid reference") || strings.Contains(msg, "unknown revision") || strings.Contains(msg, "not a valid object name")
}

// sanitizeBranchForPathUsecase mirrors localgit.sanitizeBranchForPath — this
// package cannot import an internal/adapter/localgit unexported helper, so
// it's duplicated here as a small, obviously-equivalent function rather than
// exporting one across a layer boundary this package doesn't otherwise
// depend on.
func sanitizeBranchForPathUsecase(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}
