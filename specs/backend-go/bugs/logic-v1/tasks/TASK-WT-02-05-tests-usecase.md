# TASK-WT-02-05: Tests for `FanOutCreateWorktrees.Execute`/`runOne` (BR-WT-05/06/07/08)

**From Solution:** SOL-WT-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/usecase/fan_out_create_worktrees_test.go` (new)
**Depends on:** TASK-WT-02-02
**Status:** `[x]` DONE — fan_out_create_worktrees_test.go created with all 6 cases; `go test ./internal/usecase/... -run FanOutCreateWorktrees -race` passes

---

## Context

Per [SOL-WT-02](../solutions/SOL-WT-02-fan-out-worktree.md)'s Test plan, with `_OneItemFails_OthersStillComplete` as the core regression guard against copying `worktree.detectedList`'s `errgroup.WithContext` pattern (which would violate BR-WT-08).

## Changes to make

Create `backend-go/services/api-gateway/internal/usecase/fan_out_create_worktrees_test.go` with fake `WorktreeCreator`/`AgentSpawner`/`PromptInjector` implementations (guarded by a `sync.Mutex` since `Execute` runs items concurrently) and these cases:

- `TestFanOutCreateWorktrees_RejectsN0AndN11_NoCallsMade` — `N=0` and `N=11` both return `FANOUT_N_OUT_OF_RANGE`; assert the fake `WorktreeCreator` recorded zero calls.
- `TestFanOutCreateWorktrees_AllNShareSameBaseRef` — `N=5`; assert every recorded `CreateWorktree` call received the identical `baseRef` (BR-WT-06 by construction).
- `TestFanOutCreateWorktrees_OneItemFails_OthersStillComplete` — fake `WorktreeCreator` fails only when `branch` matches index 2's derived branch name; assert `results[0]`, `results[1]`, `results[3:]` are all `"ready"` and no result carries a context-cancellation error (BR-WT-08 — the core regression guard).
- `TestFanOutCreateWorktrees_PromptInjectedOnlyAfterSpawnSucceeds` — fake `AgentSpawner` records a monotonic call sequence number; fake `PromptInjector` asserts its own call's sequence number is strictly greater (BR-WT-07 ordering).
- `TestFanOutCreateWorktrees_SpawnFails_PromptInjectorNeverCalled` — fake `AgentSpawner` errors for one index; assert the fake `PromptInjector` recorded zero calls for that index.
- `TestFanOutCreateWorktrees_RetrySingleIndex_ViaRunOne` — call `uc.RunOne` directly for a previously-failed index; assert the other indices' fakes recorded no additional calls.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/api-gateway/internal/usecase/... -run FanOutCreateWorktrees -race
```

Expected: all cases pass, including under `-race` — this usecase's whole point is concurrent per-item isolation, so a data race here is a real regression, not test flakiness.
