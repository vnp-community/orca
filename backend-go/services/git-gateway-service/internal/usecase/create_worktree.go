package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef, Name, Path string
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
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewCreateWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *CreateWorktree {
	return &CreateWorktree{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
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

	// dispatchExecutor's key is the repo confirmed by GetRepo (repo.ID),
	// not the raw request field — see ports.go's dispatchExecutor doc
	// comment for why that distinction matters here.
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
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
	onDisk, _ := executor.ListWorktreePaths(ctx, repoPath)
	taken := make(map[string]bool, len(onDisk))
	for _, p := range onDisk {
		taken[p] = true
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
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "git worktree add failed", err)
	}

	worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch, in.BaseRef)
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
