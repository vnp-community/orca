# TASK-PW-03-04: `localgit.Executor` implements merge/stash/branch-create/soft-delete

**From Solution:** SOL-PW-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go`
**Depends on:** TASK-PW-03-03
**Status:** `[x]` DONE — localgit.Executor implements MergeBranch/StashPush/StashPop/CreateBranch/DeleteBranch against real temp git repos; 7 new integration tests pass incl. genuine-conflict and unmerged-branch-delete-fails cases

---

## Context

Host-local case — always supported, no connection-mode gate applies here
(that gate lives in the usecase layer, TASK-PW-03-06). All five
subcommands (`merge`/`stash`/`branch`) are baseline-compatible per
`docs/reference/git-compatibility.md`'s Git 2.25 floor — no
`GitCapabilityCache` fallback needed for this addition.

## Changes to make

Add to `internal/adapter/localgit/executor.go`, following this file's
existing `os/exec`-backed pattern (see `ForceDeleteBranch`/
`ListLocalBranches` at lines 535/572 for the shape to match):

```go
func (e *Executor) MergeBranch(ctx context.Context, repoPath, branch string, noFF bool) (domain.MergeResult, error) {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		return domain.MergeResult{Success: false, HadConflicts: strings.Contains(out, "CONFLICT")}, nil
	}
	return domain.MergeResult{Success: true}, nil
}

func (e *Executor) StashPush(ctx context.Context, repoPath, message string, includeUntracked bool) (domain.SimpleResult, error) {
	args := []string{"stash", "push"}
	if includeUntracked {
		args = append(args, "-u")
	}
	if message != "" {
		args = append(args, "-m", message)
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

func (e *Executor) StashPop(ctx context.Context, repoPath, stashRef string) (domain.MergeResult, error) {
	args := []string{"stash", "pop"}
	if stashRef != "" {
		args = append(args, stashRef)
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		return domain.MergeResult{Success: false, HadConflicts: strings.Contains(out, "CONFLICT")}, nil
	}
	return domain.MergeResult{Success: true}, nil
}

func (e *Executor) CreateBranch(ctx context.Context, repoPath, branch, baseRef string, checkout bool) (string, error) {
	args := []string{"branch", branch}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return "", err
	}
	if checkout {
		if _, err := e.run(ctx, repoPath, "checkout", branch); err != nil {
			return "", err
		}
	}
	return branch, nil
}

func (e *Executor) DeleteBranch(ctx context.Context, repoPath, branch string) error {
	_, err := e.run(ctx, repoPath, "branch", "-d", branch)
	return err
}
```

Check this file's existing helper name for running `git` and capturing
combined output/exit status (used above as `e.run`) before writing this —
`ForceDeleteBranch`/`ListLocalBranches` already call something with this
shape; match its exact name and error-wrapping convention rather than
inventing a new one. Add the `"strings"` import if not already present.

Add real temp-repo integration tests to `executor_test.go`: merge with a
genuine conflict reports `HadConflicts=true`; stash push/pop round-trip
restores working-tree state; create-branch-with-checkout leaves HEAD on
the new branch; soft-delete of an unmerged branch fails (matching `git
branch -d`'s own safety behavior, distinct from `-D`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/adapter/localgit/... -run 'TestMergeBranch|TestStashPush|TestStashPop|TestCreateBranch|TestDeleteBranch' -v
```

Expected: clean build; all five new integration tests pass against a real
temp git repo.
