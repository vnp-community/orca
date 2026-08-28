# TASK-WT-05-03: `localgit.Executor.MergeBranch` implementation

**From Solution:** SOL-WT-05
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go`
**Depends on:** TASK-WT-05-02
**Status:** `[x]` DONE — localgit.Executor.MergeBranch (merge --no-ff / --squash+commit) + conflictedPaths helper (git diff --name-only --diff-filter=U, no pre-existing helper found); RelayExecutor.MergeBranch stub relaying to git.merge. go build clean.

---

## Context

Real `git merge`/`git merge --squash` execution. Per [SOL-WT-05](../solutions/SOL-WT-05-merge-worktree.md), a conflict is git exiting non-zero with `CONFLICT` markers in its output — an expected domain outcome BR-WT-17 requires surfacing, not a Go error, mirroring `RebaseFromBase`'s existing conflict-detection posture (`executor.go:628-637`).

## Changes to make

Add to `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go` (near `RebaseFromBase`, or the local package's merge-related helpers):

```go
// MergeBranch runs `git merge --no-ff <branch>` ("merge" strategy) or
// `git merge --squash <branch>` followed by a commit ("squash" strategy).
// "rebase" is handled entirely by the caller composing RebaseFromBase+
// FastForward — this method is never called for that strategy, see
// MergeWorktreeIntoBase.Execute.
func (e *Executor) MergeBranch(ctx context.Context, repoPath, branch, strategy, commitMessage string) (domain.MergeResult, error) {
	args := []string{"merge"}
	switch strategy {
	case "merge":
		args = append(args, "--no-ff", branch)
	case "squash":
		args = append(args, "--squash", branch)
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		if strings.Contains(out, "CONFLICT") || strings.Contains(err.Error(), "CONFLICT") {
			paths, _ := e.conflictedPaths(ctx, repoPath)
			return domain.MergeResult{HasConflicts: true, ConflictedPaths: paths}, nil
		}
		return domain.MergeResult{}, err
	}
	if strategy == "squash" {
		msg := commitMessage
		if msg == "" {
			msg = "Squash merge " + branch
		}
		if _, err := e.run(ctx, repoPath, "commit", "-m", msg); err != nil {
			return domain.MergeResult{}, err
		}
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.MergeResult{}, err
	}
	return domain.MergeResult{ResultSHA: strings.TrimSpace(sha)}, nil
}
```

Before writing `conflictedPaths`, check whether `ConflictOperation`'s existing implementation (`executor.go`, search `func (e *Executor) ConflictOperation`) already has a marker-file-scan helper this can reuse instead of duplicating one:

```bash
cd /opt/repos/orca/backend-go
grep -n "func (e \*Executor) ConflictOperation\|func (e \*Executor) conflictedPaths\|func (e \*Executor) ResolveConflict" services/git-gateway-service/internal/adapter/localgit/executor.go
```

If no such helper exists yet, add a small one that runs `git diff --name-only --diff-filter=U` (lists unmerged/conflicted paths) rather than a new marker-file scan — cheaper and matches what git itself considers "conflicted."

Add a minimal `RelayExecutor.MergeBranch` in `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`, following this file's existing `relay(...)` helper pattern (see `CreateWorktree`'s implementation, `relay_executor.go:157-162`) — relays to a `git.merge` method name; flag this as unverified against a real Dev Server Agent handler, matching this file's own existing doc-comment caveat for `CreateWorktree`/`RemoveWorktree`/etc.:

```go
func (r *RelayExecutor) MergeBranch(ctx context.Context, repoPath, branch, strategy, commitMessage string) (domain.MergeResult, error) {
	var result domain.MergeResult
	err := r.relay(ctx, repoPath, "git.merge", map[string]any{
		"repoPath": repoPath, "branch": branch, "strategy": strategy, "commitMessage": commitMessage,
	}, &result)
	return result, err
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build; `localgit.Executor` and `RelayExecutor` both satisfy `GitExecutor` again. Integration tests land in [TASK-WT-05-06](./TASK-WT-05-06-tests-usecase-and-executor.md).
