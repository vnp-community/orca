# TASK-PW-03-03: `ErrGitOpUnsupportedOverSSHRelay` sentinel + `GitExecutor` port additions

**From Solution:** SOL-PW-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/domain/domain.go`, `backend-go/services/git-gateway-service/internal/usecase/ports.go`
**Depends on:** TASK-PW-03-01, TASK-PW-03-02
**Status:** `[x]` DONE — ErrGitOpUnsupportedOverSSHRelay + domain.MergeResult added; GitExecutor interface extended with the 5 new methods; domain package builds clean (usecase compiles once 03-04/05 land, as expected)

---

## Context

This task adds the typed error the five new usecases (TASK-PW-03-06) fail
closed with over a `relay-ssh` connection, plus the `GitExecutor`
interface methods both `localgit` (TASK-PW-03-04) and `grpcclient`
(TASK-PW-03-05) must implement. Follows the exact precedent
`ErrForceDeleteBranchUnsupported`/`ErrConflictResolveUnsupportedOverRelay`
already set in `domain.go:14-41`.

## Changes to make

In `internal/domain/domain.go`, add (alongside the two existing sentinels):

```go
// ErrGitOpUnsupportedOverSSHRelay is returned when a merge/stash/branch-
// write or push/pull-progress-stream operation is attempted against a
// relay-ssh-mode connection — the real agent's git.exec whitelist for Part
// B (the surface RelayExecutor's SSH-relay calls reach) explicitly
// excludes merge/rebase/stash and has no execStream equivalent at all
// (specs/agent/api/agent-rpc-catalog-git-fs.md's "Not allowed at all"
// list). Same operational-fallback shape as ErrForceDeleteBranchUnsupported
// above — lives in domain so both grpcclient (which returns it) and
// usecase (which checks it via errors.Is) can import it without an import
// cycle.
var ErrGitOpUnsupportedOverSSHRelay = errors.New(
	"git-gateway-service: this operation requires a relay-websocket or " +
		"direct-websocket connection; relay-ssh's git.exec whitelist does " +
		"not permit merge/stash/branch-write subcommands")
```

In `internal/usecase/ports.go`'s `GitExecutor` interface, add:

```go
MergeBranch(ctx context.Context, repoPath, branch string, noFF bool) (domain.MergeResult, error)
StashPush(ctx context.Context, repoPath, message string, includeUntracked bool) (domain.SimpleResult, error)
StashPop(ctx context.Context, repoPath, stashRef string) (domain.MergeResult, error) // reuses MergeResult's had_conflicts shape
CreateBranch(ctx context.Context, repoPath, branch, baseRef string, checkout bool) (string, error)
DeleteBranch(ctx context.Context, repoPath, branch string) error // soft; ForceDeleteBranch (existing) stays the -D path
```

Add the new `MergeResult` domain type to `internal/domain/domain.go`
(alongside `PullResult`/`RebaseResult`, which it mirrors):

```go
// MergeResult reflects whether a MergeBranch (or StashPop, which reuses
// this shape) operation succeeded, and whether it left the worktree with
// unresolved conflicts — same Success/HadConflicts shape as PullResult/
// RebaseResult, since a merge conflict is a real domain outcome, not a Go
// error.
type MergeResult struct {
	Success      bool
	HadConflicts bool
}
```

This task deliberately does NOT implement any of the five methods on
either `localgit.Executor` or `grpcclient.RelayExecutor` — that would
leave the package non-compiling between this task and TASK-PW-03-04/05.
Since Go requires every `GitExecutor` implementation to satisfy the
interface immediately, land this task together with (or immediately
before, in the same PR as) TASK-PW-03-04 and TASK-PW-03-05 rather than
merging it alone.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/internal/domain/...
go vet ./services/git-gateway-service/internal/usecase/...
```

Expected: `domain` package builds clean. `usecase` package will NOT
compile in isolation until `localgit`/`grpcclient` implement the five new
`GitExecutor` methods — this is expected; `go vet`/`go build` on the full
service is the real gate, run after TASK-PW-03-04/05 land.
