# TASK-PW-03-05: `grpcclient.RelayExecutor` implements merge/stash/branch-create/soft-delete via `git.exec`

**From Solution:** SOL-PW-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`
**Depends on:** TASK-PW-03-03
**Status:** `[ ]` TODO

---

## Context

Relay case — reachable only when the target connection is
`relay-websocket`/`direct-websocket` (Part A). The usecase layer
(TASK-PW-03-06) checks `ResolvedConnection.Mode` and is expected to
prevent this method from ever being called against a `relay-ssh`
connection — matching `RenameFile`/`CopyFile`'s existing "usecase checks,
executor trusts" division of responsibility. This method does not
re-check mode itself.

## Changes to make

Add to `internal/adapter/grpcclient/relay_executor.go`, reusing the
existing `gitExecResult{Stdout, Stderr}` type `ListLocalBranches` already
defined (`relay_executor.go:570-577` per that method's own doc comment —
verify the exact type name there before reusing it):

```go
// MergeBranch relays via git.exec's merge subcommand — only reachable
// when the target connection is Part A (relay-websocket/direct-websocket);
// Part B's (relay-ssh) git.exec whitelist rejects `merge` outright. The
// usecase layer's ConnectionResolver check is expected to prevent this
// method from ever being called against a relay-ssh connection.
func (r *RelayExecutor) MergeBranch(ctx context.Context, repoPath, branch string, noFF bool) (domain.MergeResult, error) {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	var result gitExecResult
	err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result)
	if err != nil {
		return domain.MergeResult{}, err
	}
	return domain.MergeResult{Success: true, HadConflicts: strings.Contains(result.Stderr, "CONFLICT")}, nil
}

func (r *RelayExecutor) StashPush(ctx context.Context, repoPath, message string, includeUntracked bool) (domain.SimpleResult, error) {
	args := []string{"stash", "push"}
	if includeUntracked {
		args = append(args, "-u")
	}
	if message != "" {
		args = append(args, "-m", message)
	}
	var result gitExecResult
	if err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

func (r *RelayExecutor) StashPop(ctx context.Context, repoPath, stashRef string) (domain.MergeResult, error) {
	args := []string{"stash", "pop"}
	if stashRef != "" {
		args = append(args, stashRef)
	}
	var result gitExecResult
	err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result)
	if err != nil {
		return domain.MergeResult{}, err
	}
	return domain.MergeResult{Success: true, HadConflicts: strings.Contains(result.Stderr, "CONFLICT")}, nil
}

// CreateBranch composes two git.exec calls (branch then checkout)
// sequentially when checkout=true — `checkout -b`'s combined form is not
// confirmed on either Part's exec whitelist as a single flag-shape;
// verify against specs/agent/api/agent-rpc-catalog-git-fs.md before
// implementation, and prefer a single call if it IS allowlisted.
func (r *RelayExecutor) CreateBranch(ctx context.Context, repoPath, branch, baseRef string, checkout bool) (string, error) {
	args := []string{"branch", branch}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	var result gitExecResult
	if err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result); err != nil {
		return "", err
	}
	if checkout {
		var coResult gitExecResult
		if err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": []string{"checkout", branch}, "cwd": repoPath}, &coResult); err != nil {
			return "", err
		}
	}
	return branch, nil
}

func (r *RelayExecutor) DeleteBranch(ctx context.Context, repoPath, branch string) error {
	var result gitExecResult
	return r.relay(ctx, repoPath, "git.exec", map[string]any{"args": []string{"branch", "-d", branch}, "cwd": repoPath}, &result)
}
```

Add contract tests to `relay_executor_test.go` asserting the exact
`args` slice sent to `git.exec` for each of the five ops (against the
agent's real subcommand shape, not just "some call happened").

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/adapter/grpcclient/... -run 'TestMergeBranch|TestStashPush|TestStashPop|TestCreateBranch|TestDeleteBranch' -v
```

Expected: clean build (this task plus TASK-PW-03-04 together satisfy the
`GitExecutor` interface TASK-PW-03-03 extended); contract tests pass.
