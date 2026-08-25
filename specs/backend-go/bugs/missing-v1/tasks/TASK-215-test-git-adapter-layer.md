# TASK-215: Adapter-layer tests for Groups A-E (`localgit`, `grpcclient.RelayExecutor`, `adapter/grpc.Server`)

**From Solution:** SOL-032 (Test plan — `adapter/localgit/executor_test.go`, `adapter/grpcclient/relay_executor_test.go`, `adapter/grpc/server_test.go`)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/localgit/executor_test.go`, `internal/adapter/grpcclient/grpcclient_test.go`, `internal/adapter/grpc/server_test.go`
**Depends on:** TASK-207, TASK-208, TASK-209, TASK-210, TASK-211
**Status:** `[partial]` `localgit.Executor` tests added against a real `git` binary for every new method this pass implemented (Stage/Unstage, History incl. `-limit`, CheckIgnored, ForkSync, UpstreamStatus, RemoteCommitURL/RemoteFileURL incl. GitHub-HTTPS/GitHub-SSH/Bitbucket URL forms) in `executor_test.go`. `RelayExecutor` tests added for wire-shape correctness (worktreePath/filePath param keys, bulk-method targeting for Stage/Unstage, cursor-dropped/baseRef-renamed History, ignored-subset CheckIgnored, expectedUpstream ForkSync) plus a `FilesystemExecutor`-satisfied compile-time guard, in `grpcclient_test.go`. `localfs.Executor` gets its own full test file (12 tests: read/write round-trip, path-escape rejection, stat, rename, copy, createDir no-clobber, glob incl. empty-pattern-matches-everything and maxResults early-stop, search incl. `.git` exclusion). Only covers what this pass implemented — no adapter tests for TASK-207's branch/ref group or TASK-209/210's 6 BLOCKED methods. `go test ./internal/adapter/...` passes (15+12+14 = 41 tests across localgit/localfs/grpcclient).

---

## Context

Three adapter layers need coverage for every method added across Groups
A-E: the real `git`-binary executor, the relay-to-Dev-Server-Agent client,
and the gRPC request/response translation layer. Per SOL-032's test plan,
each follows an existing, established pattern in this codebase — no new
test infrastructure.

## Changes to make

### Step 1: `internal/adapter/localgit/executor_test.go` — real-git-binary fixtures

Extend using `initRepo`'s existing helper (`executor_test.go:18-43`) — a
temp repo per test, real `git` binary, no fakes. Add tests for:

- `Checkout` — checkout an existing branch; `Checkout` with `create=true` on
  a new branch name; assert `CheckoutResult.Branch` matches.
- `ListLocalBranches` — create 2 branches, set an upstream via
  `git branch --set-upstream-to`, assert `IsCurrent`/`Upstream` are correct.
- `FastForward` — create a branch ahead of `main`, fast-forward `main` to
  it, assert success; assert a non-ff case returns a Go error (diverged
  histories).
- `RebaseFromBase` (clean case) — rebase a branch with no conflicting
  changes onto an updated base, assert `Success=true, HadConflicts=false`.
- `RebaseFromBase` (conflict case) — **construct a real conflict fixture**:
  two branches editing the same line of the same file, rebase one onto the
  other, assert `HadConflicts=true` against real `git` state, not a mocked
  one (per SOL-032's explicit instruction).
- `AbortRebase` — start the conflict fixture above, call `AbortRebase`,
  assert the worktree returns to its pre-rebase state (`git status` clean).
- `AbortMerge` — construct a real merge-conflict fixture (same two-branch
  technique, `git merge` instead of `git rebase`), call `AbortMerge`,
  assert clean state after.
- `ConflictOperation` — using the merge-conflict fixture, test all 3
  operations (`ours`, `theirs`, `markResolved`); assert the file's final
  content and that it's staged after each.
- `Discard` — one test for a tracked-file discard (modify then discard,
  assert content reverts), one for an untracked-file discard (create then
  discard, assert the file is gone).
- `BulkDiscard` — mix a valid path and a nonexistent path in one call,
  assert `Success=false` and the nonexistent path appears in
  `FailedPaths` while the valid path's discard still took effect.
- `Stage`/`Unstage` — stage a new file, assert `git status --porcelain`
  shows it staged; unstage it, assert it's back to untracked/modified.
- `History` — create 3 commits, call with `limit=2`, assert exactly 2
  commits returned newest-first and `NextCursor` equals the 2nd commit's
  SHA; call again with that cursor, assert the 3rd (oldest) commit is
  returned.
- `CommitCompare`/`BranchCompare` — create a base commit and 2 more commits
  on a branch, assert both the commit list and `FilesChanged` are correct.
- `CommitDiff`/`BranchDiff` — assert the returned `UnifiedDiff` contains the
  expected changed line.
- `SubmoduleStatus` — a repo with no submodules returns an empty slice (no
  error); if adding a real submodule fixture is impractical in CI, assert
  the empty-slice case only and leave a dirty-submodule case as a
  documented follow-up.
- `CheckIgnored` — add a `.gitignore` with one pattern, assert the matching
  path comes back `Ignored=true` and a non-matching path `Ignored=false`.
- `ForkSync`/`UpstreamStatus` — these need a real second remote (a bare
  repo in another temp dir, added via `git remote add`); assert ahead/
  behind counts after pushing divergent commits to each side.
- `Fetch` — fetch from the same bare-repo-remote fixture, assert success
  and that the remote-tracking ref updated.
- `RemoteCommitURL`/`RemoteFileURL` — set `origin` to a few representative
  URL forms (`https://github.com/org/repo.git`, `git@gitlab.com:org/repo.git`)
  and assert the resolved URL matches the expected GitHub/GitLab
  `/commit/<sha>` and `/blob/<ref>/<path>` shape; one Bitbucket case
  asserting the `/src/<ref>/<path>` shape from `RemoteFileURL`.

### Step 2: `internal/adapter/grpcclient/grpcclient_test.go` — `RelayExecutor` method coverage

Extend `fakeInfraFleetServiceClient`'s assertions (already-established
pattern, `grpcclient_test.go:15-46`) with one test per new `RelayExecutor`
method — verify each sends the correct `Relay.Method` string (e.g.
`"git.checkout"`, `"git.stage"`, `"git.history"`) and the expected
JSON param shape, and correctly unmarshals a fake `ResultJson` into the
matching domain type. Representative example:

```go
func TestRelayExecutor_Checkout_SendsExpectedMethodAndParams(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayResp: &infrafleetv1.RelayResponse{ResultJson: `{"success":true,"branch":"feature/x"}`},
	}
	r := NewRelayExecutor(fake)

	got, err := r.Checkout(ctxWithTenant(t), "/repo/wt1", "feature/x", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotRelay.GetMethod() != "git.checkout" {
		t.Errorf("expected method=git.checkout, got %q", fake.gotRelay.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(fake.gotRelay.GetParamsJson()), &params); err != nil {
		t.Fatalf("params not valid JSON: %v", err)
	}
	if params["ref"] != "feature/x" || params["create"] != true {
		t.Errorf("unexpected params: %+v", params)
	}
	if !got.Success || got.Branch != "feature/x" {
		t.Errorf("unexpected result: %+v", got)
	}
}
```

Repeat for all 22 new `RelayExecutor` methods across Groups A-D
(`ListLocalBranches`, `FastForward`, `RebaseFromBase`, `AbortRebase`,
`AbortMerge`, `ConflictOperation`, `Discard`, `BulkDiscard`, `Stage`,
`Unstage`, `History`, `CommitCompare`, `BranchCompare`, `CommitDiff`,
`BranchDiff`, `SubmoduleStatus`, `CheckIgnored`, `ForkSync`,
`UpstreamStatus`, `Fetch`, `RemoteCommitURL`, `RemoteFileURL`) — no live Dev
Server Agent needed. `GeneratePullRequestFields`'s relay leg (TASK-211)
reuses the already-tested `AICompleter.Complete` path, so it needs no new
`RelayExecutor` test here; add one test for
`AIProviderResolver.ResolveProvider` (a new fake
`aiproviderv1.AiProviderServiceClient`, following the same
embed-and-override-methods pattern as `fakeInfraFleetServiceClient`).

### Step 3: `internal/adapter/grpc/server_test.go` — contract tests

One request→usecase→response test per new RPC, per
`03-clean-architecture-guidelines.md`'s `adapter/grpc` contract-test
policy — following this file's existing per-RPC test shape for
`GetStatus`/`Commit`/etc. (fake usecase execute, assert the proto response
translates the usecase result's fields 1:1). 33 new RPCs total across
TASK-207 through TASK-211.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go test ./internal/adapter/... -count=1 -v
```

Expected: all new tests pass; `RebaseFromBase`'s and `AbortMerge`'s
conflict-fixture tests in particular exercise real `git` conflict state,
not a mocked outcome.
