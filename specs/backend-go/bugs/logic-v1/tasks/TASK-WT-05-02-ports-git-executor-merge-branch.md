# TASK-WT-05-02: Add `GitExecutor.MergeBranch` to the port

**From Solution:** SOL-WT-05
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go`
**Depends on:** TASK-WT-05-01
**Status:** `[x]` DONE — Added GitExecutor.MergeBranch + domain.MergeResult; go build clean once localgit/relay/fakes implement it (done together with WT-05-03 rather than as a temporary stub, since both landed in this same pass).

---

## Context

Declares the interface method both `localgit.Executor` ([TASK-WT-05-03](./TASK-WT-05-03-localgit-merge-branch.md)) and `RelayExecutor` must implement before the merge usecase ([TASK-WT-05-04](./TASK-WT-05-04-usecase-merge-worktree.md)) can compile against it. Per [SOL-WT-05](../solutions/SOL-WT-05-merge-worktree.md): this method is only ever called for the "merge"/"squash" strategies — "rebase" is handled entirely by the usecase composing the already-real `RebaseFromBase`+`FastForward`.

## Changes to make

Add to `GitExecutor` in `backend-go/services/git-gateway-service/internal/usecase/ports.go` (near `AbortMerge`, `ports.go:225`):

```go
	// MergeBranch runs `git merge --no-ff <branch>` ("merge" strategy) or
	// `git merge --squash <branch>` followed by a commit ("squash"
	// strategy). NEVER called for the "rebase" strategy — see
	// MergeWorktreeIntoBase.Execute, which composes RebaseFromBase+FastForward
	// for that case instead. A real conflict is a domain outcome
	// (HasConflicts=true), not a Go error — same posture as
	// RebaseFromBase/Pull's conflict handling.
	MergeBranch(ctx context.Context, repoPath, branch, strategy, commitMessage string) (domain.MergeResult, error)
```

Add `domain.MergeResult` to `backend-go/services/git-gateway-service/internal/domain/domain.go`:

```go
// MergeResult reflects a MergeBranch operation's outcome. A conflict is
// reported via HasConflicts, not an error — the repo is left in the
// conflicted state for the client to resolve via the existing
// ConflictOperation/ResolveConflict/AbortMerge RPCs (BR-WT-17: manual
// resolution only, never auto-resolved or auto-aborted).
type MergeResult struct {
	ResultSHA           string
	HasConflicts        bool
	ConflictedPaths     []string
	ConflictDispatchKey string
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: build fails until every `GitExecutor` implementation (real and fake) adds `MergeBranch` — `localgit.Executor` in [TASK-WT-05-03](./TASK-WT-05-03-localgit-merge-branch.md), `RelayExecutor` and test fakes fixed as part of this task so the interface addition alone doesn't break the build tree for longer than necessary (a minimal `RelayExecutor.MergeBranch` stub returning `apperrors`-free zero values is acceptable here if [TASK-WT-05-03](./TASK-WT-05-03-localgit-merge-branch.md) hasn't landed yet — replace with a real relay call in that task).
