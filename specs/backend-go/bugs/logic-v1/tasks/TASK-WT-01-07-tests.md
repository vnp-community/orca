# TASK-WT-01-07: Tests for BR-WT-01/04, [A1]/[A2]/[A3], and the outbox event

**From Solution:** SOL-WT-01
**Priority:** P1
**Service:** `git-gateway-service` + `project-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/create_worktree_test.go`
**Depends on:** TASK-WT-01-02, TASK-WT-01-03, TASK-WT-01-04, TASK-WT-01-05, TASK-WT-01-06
**Status:** `[x]` DONE — Added worktree_name_test.go, create_worktree_test.go new cases (invalid name/path exists/limit/base-ref-not-found/custom path), executor_test.go TestCreateWorktree_ExplicitTargetPath+TestCheckFreeSpace, record_worktree_created_test.go outbox assertion, worktree_repository_test.go outbox integration test. All pass (go test + integration -tags integration with real Postgres).

---

## Context

Regression coverage for every rule [SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md) adds, per its own Test plan section.

## Changes to make

`backend-go/services/git-gateway-service/internal/domain/worktree_name_test.go` (new):

```go
package domain_test

import (
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestValidateWorktreeName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
	}{
		{"lowercase-dash-underscore-digits", "feature-123_abc", true},
		{"rejects-uppercase", "Feature", false},
		{"rejects-spaces", "my feature", false},
		{"rejects-unicode", "fïx", false},
		{"rejects-empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateWorktreeName(tc.input)
			if tc.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("expected invalid, got nil")
			}
		})
	}
}

func TestSuggestAlternateName_WalksCollisions(t *testing.T) {
	taken := map[string]bool{"foo": true, "foo-2": true}
	if got := domain.SuggestAlternateName("foo", taken); got != "foo-3" {
		t.Fatalf("got %q, want foo-3", got)
	}
	if got := domain.SuggestAlternateName("bar", taken); got != "bar" {
		t.Fatalf("got %q, want bar (untaken)", got)
	}
}
```

Add to `backend-go/services/git-gateway-service/internal/usecase/create_worktree_test.go` (table-style cases, matching this file's existing fake-based test conventions — see `worktree_fakes_test.go` for the fake `ProjectClient`/`GitExecutor` shapes already in the package):

- `TestCreateWorktree_InvalidName_RejectsBeforeAnyExecutorCall` — assert zero calls recorded on the fake `local`/`relay` executors.
- `TestCreateWorktree_PathAlreadyExists_ReturnsSuggestedName_NoGitCallAttempted` — fake `ListWorktreePaths` returns a path colliding with the derived target; assert the returned `apperrors.AppError`'s code is `WORKTREE_PATH_EXISTS` and the fake executor's `CreateWorktree` was never called.
- `TestCreateWorktree_LimitExceeded_RejectsBeforeGitCall` — fake `ProjectClient.ListWorktrees` returns 20 active rows for `in.RepoID`.
- `TestCreateWorktree_LimitCheckFailsOpen_WhenListWorktreesErrors` — fake `ListWorktrees` returns an error; assert creation still proceeds.
- `TestCreateWorktree_BaseRefNotFound_ClassifiesGitError_AttachesBranchList` — fake executor's `CreateWorktree` returns an error containing `"invalid reference"`; fake `ListLocalBranches` returns branch names; assert the error message lists them.
- `TestCreateWorktree_CustomNameAndPath_PassedThroughToExecutor` — assert the fake executor's recorded `CreateWorktree` call received `in.Path` verbatim as `targetPath`.
- Update every existing happy-path/compensation test in this file for the new `targetPath` param on the fake `GitExecutor.CreateWorktree` signature.

`backend-go/services/git-gateway-service/internal/adapter/localgit/executor_test.go` — add:
- `TestCreateWorktree_ExplicitTargetPath` (integration, real git in a temp repo) — asserts the worktree lands at the explicit `targetPath`, not the derived default.
- `TestCheckFreeSpace` — unit test with a real temp directory (statfs against a live filesystem is fine here; no stub needed since `unix.Statfs` works against any real path).

`backend-go/services/project-service/internal/usecase/record_worktree_created_test.go` — add `TestRecordWorktreeCreated_WritesOutboxEventInSameTransaction`: fake `WorktreeRepository.RecordWorktreeCreated` captures its `event domain.OutboxEvent` arg; assert `event.Subject == "orca.project.worktree.created"` and `event.PayloadJSON` round-trips to the expected worktree id/project id/repo id/path/branch.

`backend-go/services/project-service/internal/adapter/postgres/worktree_repository_test.go` — add `TestRecordWorktreeCreated_OutboxRowCommittedWithWorktreeRow` (integration, against the test Postgres instance this package's existing tests already use): after a successful call, query `project.outbox_events` directly and assert exactly one unpublished row exists with the expected `subject`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/git-gateway-service/internal/domain/... ./services/git-gateway-service/internal/usecase/... ./services/git-gateway-service/internal/adapter/localgit/...
go test ./services/project-service/internal/usecase/... ./services/project-service/internal/adapter/postgres/...
```

Expected: all new and existing tests pass.
