# TASK-CR-04-02: Split `gatherFullDiff` so `Execute` can branch before spending `GetDiff` calls

**From Solution:** SOL-CR-04
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/diff_composer.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BR-CR-15's size-based fallback needs to know `status.Files`' count
*before* deciding whether to spend N `GetDiff` round trips at all —
avoiding the N extra dispatch calls entirely above the threshold, not just
a truncated diff. `gatherFullDiff` currently calls `getStatus.Execute`
itself, so its caller can't inspect the file count first. This task splits
status-fetching out into a thin wrapper so both the pre-existing
`gatherFullDiff(ctx, getStatus, getDiff, worktreeID, staged)` call shape
(used by `GeneratePullRequestFields`) and TASK-CR-04-03's new
threshold-checking `Execute` keep working.

## Changes to make

Replace `diff_composer.go`'s body:

```go
package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// gatherFullDiff composes a whole-worktree unified diff by listing changed
// files via GetStatus and concatenating one per-file GetDiff call per file
// — GetDiff itself is per-file only (TASK-228: the real Dev Server Agent's
// git.diff has no whole-repo mode), but GenerateCommitMessage/
// GeneratePullRequestFields genuinely need full context for a useful
// prompt, so that composition happens here, once, entirely inside
// git-gateway-service.
func gatherFullDiff(ctx context.Context, getStatus *GetStatus, getDiff *GetDiff, worktreeID string, staged bool) (string, error) {
	status, err := getStatus.Execute(ctx, GetStatusInput{WorktreeID: worktreeID})
	if err != nil {
		return "", err
	}
	return gatherFullDiffFromStatus(ctx, getDiff, worktreeID, status, staged)
}

// gatherFullDiffFromStatus is gatherFullDiff's body minus its own
// GetStatus.Execute call, so a caller that already has a GitStatus (e.g.
// GenerateCommitMessage.Execute, which needs status.Files' count for
// BR-CR-15's threshold check before deciding whether to call this at all)
// doesn't pay for a second resolve-and-dispatch round trip.
func gatherFullDiffFromStatus(ctx context.Context, getDiff *GetDiff, worktreeID string, status domain.GitStatus, staged bool) (string, error) {
	var b strings.Builder
	for _, f := range status.Files {
		diff, err := getDiff.Execute(ctx, GetDiffInput{WorktreeID: worktreeID, FilePath: f.Path, Staged: staged})
		if err != nil {
			return "", err
		}
		b.WriteString(diff.UnifiedDiff)
	}
	return b.String(), nil
}
```

`GetStatus.Execute` returns `domain.GitStatus` (confirmed against
`get_status.go`), not a wrapper result type — the signature above uses
that real type directly.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./...
go test ./internal/usecase/... -run TestGeneratePullRequestFields -v
```

`generate_pull_request_fields.go`'s existing call site
(`gatherFullDiff(ctx, uc.getStatus, uc.getDiff, in.WorktreeID, false)`) is
unaffected by this split — its test must still pass unchanged, confirming
no regression on the call site this task doesn't otherwise touch.
