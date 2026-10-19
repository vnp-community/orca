package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// CodeBaseRefNotFound is the code CreateWorktree and PrefetchCreateBase share for a base ref git does not know. Callers tell
// it from a failure to ask (the remote or the dev server cannot be reached) by this code alone.
const CodeBaseRefNotFound = "WORKTREE_BASE_REF_NOT_FOUND"

// maxAvailableRefsListed bounds the "available:" list of the error message: a repository can hold thousands of branches.
const maxAvailableRefsListed = 20

// PrefetchCreateBase ensures baseRef is fetched/up to date in repoID's
// local clone and returns its resolved SHA. See detect_worktrees.go's doc
// comment for why this resolves repoID via ProjectClient.GetRepo before
// dispatching, rather than passing repoID straight into dispatchExecutor.
//
// A ref git does not know fails with NotFound and WORKTREE_BASE_REF_NOT_FOUND (with the branches that do exist, like
// CreateWorktree); a remote or dev server that cannot be reached fails with Unavailable or DeadlineExceeded, so a caller can
// tell "the ref is wrong" from "could not ask".
type PrefetchCreateBase struct {
	reachability RepoHostRouting
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewPrefetchCreateBase(reachability RepoHostRouting, projects ProjectClient, local, relay GitExecutor) *PrefetchCreateBase {
	return &PrefetchCreateBase{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *PrefetchCreateBase) Execute(ctx context.Context, repoID, baseRef string) (string, error) {
	// An option-looking ref would be read by git as a flag, not as a ref.
	if strings.TrimSpace(baseRef) == "" || strings.HasPrefix(baseRef, "-") {
		return "", apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_BASE_REF_INVALID", "base_ref must be a branch or tag name", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, repoID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	sha, err := executor.FetchAndResolveRef(ctx, repoPath, baseRef)
	if err != nil {
		return "", classifyPrefetchFailure(ctx, executor, repoPath, baseRef, err)
	}
	return sha, nil
}

// classifyPrefetchFailure turns the executor's error into the one code the caller acts on. The order matters: git prints
// "Could not read from remote repository" after a missing ref as well, so "missing" is decided before "unreachable".
func classifyPrefetchFailure(ctx context.Context, executor GitExecutor, repoPath, baseRef string, cause error) error {
	switch {
	case HostDispatchError(cause) != nil:
		return HostDispatchError(cause)
	case domain.IsRemoteRefMissing(cause):
		msg := fmt.Sprintf("branch %q not found", baseRef)
		if names := suggestedRefs(ctx, executor, repoPath, baseRef); len(names) > 0 {
			msg += "; available: " + strings.Join(names, ", ")
		}
		return apperrors.New(apperrors.KindNotFound, CodeBaseRefNotFound, msg, cause)
	case errors.Is(cause, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
		return apperrors.New(apperrors.KindDeadlineExceeded, "WORKTREE_PREFETCH_TIMEOUT", "fetching the base ref timed out", cause)
	case domain.IsRemoteUnreachable(cause):
		return apperrors.New(apperrors.KindUnavailable, "WORKTREE_PREFETCH_UNAVAILABLE", "the repository's remote could not be reached", cause)
	}
	return apperrors.New(apperrors.KindInternal, "WORKTREE_PREFETCH_FAILED", "failed to prefetch base ref", cause)
}

// suggestedRefs lists the local branches, those that contain the asked name first, so the first few are the useful ones.
// A listing that fails only costs the suggestions.
func suggestedRefs(ctx context.Context, executor GitExecutor, repoPath, baseRef string) []string {
	branches, err := executor.ListLocalBranches(ctx, repoPath)
	if err != nil {
		return nil
	}
	want := strings.ToLower(baseRef)
	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return strings.Contains(strings.ToLower(names[i]), want) && !strings.Contains(strings.ToLower(names[j]), want)
	})
	if len(names) > maxAvailableRefsListed {
		names = names[:maxAvailableRefsListed]
	}
	return names
}
