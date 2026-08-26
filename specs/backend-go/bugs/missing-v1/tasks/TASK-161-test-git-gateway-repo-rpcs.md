# TASK-161: Tests for the 8 new `git-gateway-service` repo RPCs + their `wscompat` channels

**From Solution:** SOL-023 (Bucket 3 test plan)
**Priority:** P1
**Service:** `git-gateway-service` + `api-gateway`
**File:** `services/git-gateway-service/internal/usecase/{clone,init_repo,base_ref_default,search_refs,check_hooks,read_issue_command,write_issue_command,scan_setup_script_imports}_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-156, TASK-157, TASK-158, TASK-159, TASK-160
**Status:** `[ ]` TODO

---

## Changes to make

### `services/git-gateway-service/internal/usecase/clone_test.go`

Already specified in TASK-157 — table-driven, connected (relay path) vs.
not-connected (local path), fake `DevServerReachability`/`GitExecutor`.
Write it as part of this task if not already done.

### `internal/usecase/init_repo_test.go`

Same two-branch shape as `clone_test.go`, fake `DevServerReachability`
returning `true`/`false`, asserting the right `GitExecutor` (`local` vs
`relay`) receives the `InitRepo` call.

### `internal/usecase/base_ref_default_test.go` (and the same shape for `search_refs`, `check_hooks`, `read_issue_command`, `write_issue_command`, `scan_setup_script_imports`)

Mirror `get_status_test.go`'s existing shape exactly — fake
`ConnectionResolver` returning `Connected: true` (asserts the `relay`
`GitExecutor` is called) vs. `Connected: false` (asserts `local` is
called), plus a `WorktreeID: ""` case asserting
`GITGATEWAY_MISSING_WORKTREE_ID`. Example for `base_ref_default_test.go`:

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

func TestBaseRefDefault_DispatchesToResolvedExecutor(t *testing.T) {
	t.Run("missing worktree id is rejected", func(t *testing.T) {
		uc := usecase.NewBaseRefDefault(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), usecase.BaseRefDefaultInput{})
		if err == nil {
			t.Fatal("expected error for missing worktree_id")
		}
	})

	t.Run("connected worktree dispatches to relay executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{connected: true, repoPath: "/srv/repo"}
		relay := &fakeGitExecutor{baseRef: "main"}
		local := &fakeGitExecutor{}
		uc := usecase.NewBaseRefDefault(resolver, local, relay)

		ref, err := uc.Execute(context.Background(), usecase.BaseRefDefaultInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref != "main" {
			t.Errorf("got ref %q, want main", ref)
		}
		if !relay.baseRefDefaultCalled || local.baseRefDefaultCalled {
			t.Error("expected relay executor to be called, not local")
		}
	})

	t.Run("not connected dispatches to local executor", func(t *testing.T) {
		resolver := &fakeConnectionResolver{connected: false, repoPath: "/local/repo"}
		local := &fakeGitExecutor{baseRef: "main"}
		relay := &fakeGitExecutor{}
		uc := usecase.NewBaseRefDefault(resolver, local, relay)

		_, err := uc.Execute(context.Background(), usecase.BaseRefDefaultInput{WorktreeID: "wt1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !local.baseRefDefaultCalled || relay.baseRefDefaultCalled {
			t.Error("expected local executor to be called, not relay")
		}
	})
}
```

`fakeConnectionResolver`/`fakeGitExecutor` should be extended (not
duplicated) from whatever fakes `get_status_test.go` already declares in
this package — add the new tracked-call fields (`baseRefDefaultCalled`,
etc.) to the existing fake `GitExecutor` rather than writing a second one.

### `services/api-gateway/internal/adapter/wscompat/channels_test.go`

One test per new channel (`repo.clone`, `repo.baseRefDefault`,
`repo.searchRefs`, `repo.create`, `repo.hooksCheck`,
`repo.issueCommandRead`, `repo.issueCommandWrite`,
`repo.setupScriptImports`), following the same fake-`GitGatewayServiceClient`
pattern this file already uses for `registerGitChannels`'s existing
`git.status`/`git.diff` tests — assert decoded args map onto the right
gRPC request fields and the response passes through unchanged.

### Regression guard

A test asserting all 13 `repo.*` channels (4 from TASK-151, 1 from
TASK-154, 8 from TASK-160) are present in the registry — no
`notImplementedHandler` fallthrough — same style as BUG-002's sibling
reports.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/git-gateway-service/internal/usecase/... -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestRegisterRepoChannels -v
```
