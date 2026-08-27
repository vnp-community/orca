# TASK-WT-01-04: `GitExecutor.CreateWorktree` accepts an explicit target path; local disk-space check

**From Solution:** SOL-WT-01
**Priority:** P0 — the usecase task depends on this signature
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

The spec's custom `path` input (`docs/logic/worktree-management/BL-WT-01-tao-worktree.md`) needs `CreateWorktree` to target an explicit path instead of always deriving `repoPath + "-" + branch` (current `executor.go:472-482`). [SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md) also adds a local-dispatch-only [A3] disk-space warning check, since no Dev Server Agent primitive exists for it (relay dispatch skips this check entirely).

## Changes to make

`backend-go/services/git-gateway-service/internal/usecase/ports.go` — change the `GitExecutor.CreateWorktree` signature:

```go
	// CreateWorktree runs `git worktree add`. targetPath, if non-empty,
	// overrides the default repoPath+"-"+sanitize(branch) convention — see
	// SOL-WT-01's custom name/path input support.
	CreateWorktree(ctx context.Context, repoPath, branch, baseRef, targetPath string) (domain.WorktreeCreateResult, error)
```

`backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go` — update `CreateWorktree`:

```go
func (e *Executor) CreateWorktree(ctx context.Context, repoPath, branch, baseRef, targetPath string) (domain.WorktreeCreateResult, error) {
	if targetPath == "" {
		targetPath = repoPath + "-" + sanitizeBranchForPath(branch)
	}
	if _, err := e.run(ctx, repoPath, "worktree", "add", targetPath, "-b", branch, baseRef); err != nil {
		return domain.WorktreeCreateResult{}, err
	}
	sha, err := e.run(ctx, targetPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.WorktreeCreateResult{}, err
	}
	return domain.WorktreeCreateResult{Path: targetPath, HeadSHA: strings.TrimSpace(sha)}, nil
}
```

Add the [A3] disk-space check as a new file, `backend-go/services/git-gateway-service/internal/adapter/localgit/diskspace.go`:

```go
package localgit

import "golang.org/x/sys/unix"

// checkFreeSpace is [A3]'s soft warning check, local-dispatch only — relay
// dispatch has no Dev Server Agent disk-usage RPC (same absence BUG-009/
// SOL-009 documents for the agent's fs.* method set), so this is never
// called for a relay-resolved worktree. A statfs failure fails OPEN (ok=true)
// — a broken disk-space check must never block worktree creation.
func checkFreeSpace(parentDir string, minBytes uint64) (ok bool, availableBytes uint64, err error) {
	var stat unix.Statfs_t
	if statErr := unix.Statfs(parentDir, &stat); statErr != nil {
		return true, 0, statErr
	}
	available := stat.Bavail * uint64(stat.Bsize)
	return available >= minBytes, available, nil
}
```

Add a package-level constant near `sanitizeBranchForPath` in `executor.go` for the minimum threshold (500MB, a conservative default — no spec-given number exists, flagged as such):

```go
// minFreeSpaceBytes is [A3]'s soft-warning threshold — no spec-given number
// exists for "cảnh báo dung lượng disk" (BL-WT-01), 500MB is a conservative
// default worth revisiting once product specifies one.
const minFreeSpaceBytes = 500 * 1024 * 1024
```

`RelayExecutor.CreateWorktree` (`internal/adapter/grpcclient/relay_executor.go:157-162`) gains the same trailing `targetPath` param, forwarded on the wire:

```go
func (r *RelayExecutor) CreateWorktree(ctx context.Context, repoPath, branch, baseRef, targetPath string) (domain.WorktreeCreateResult, error) {
	var result domain.WorktreeCreateResult
	err := r.relay(ctx, repoPath, "git.worktreeAdd", map[string]any{
		"repoPath": repoPath, "branch": branch, "baseRef": baseRef, "targetPath": targetPath,
	}, &result)
	return result, err
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: build fails only on call sites not yet updated for the new `targetPath` param — fix `create_worktree.go` in [TASK-WT-01-05](./TASK-WT-01-05-usecase-wire-validations.md) and any fakes in `usecase/*_test.go`/`localgit/executor_test.go` as part of this task so the package compiles standalone. `checkFreeSpace`'s own unit test is added in [TASK-WT-01-07](./TASK-WT-01-07-tests.md).
