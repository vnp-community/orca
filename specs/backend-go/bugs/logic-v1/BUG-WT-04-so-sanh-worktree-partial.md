# BUG-WT-04: No multi-worktree comparison aggregation, no lines-changed metrics, no test-result/agent-summary data

**Business Logic:** [BL-WT-04](../../../../docs/logic/worktree-management/BL-WT-04-so-sanh-worktree.md) — So sánh Kết quả Giữa Các Worktrees
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A caller can fetch per-worktree diff/status/branch-compare data one worktree at a time via existing `git.*` RPCs, so a client could assemble a comparison view itself with N round trips. But there is no backend concept of "compare these worktrees against each other": no aggregation endpoint, no per-worktree lines-added/removed counts (every diff RPC returns either a raw unified-diff string or a bare changed-file count), no test-result data anywhere in backend-go, and no agent-summary-output data anywhere in backend-go — three of the four comparison columns the spec lists (file count is the only one covered) have no backend source at all.

---

## Spec summary

`BL-WT-04` describes opening a "Compare" view after N agents finish the same task in N worktrees: the system collects each worktree's diff against the base branch and shows a side-by-side table of file-change count, lines added/removed, test results, and agent summary output, letting the user pick a "winner" to hand off to `BL-WT-05`. It requires all compared worktrees to share the same base branch (BR-WT-13) and to diff from the same base SHA (BR-WT-14), and forbids auto-selecting a winner (BR-WT-15).

## What backend-go has

- Per-worktree diff: `GetDiff` RPC (`backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `GetDiffResponse { string unified_diff = 1; }`) — wired for real end-to-end (`git.diff` channel, `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:237-251`, per `BUG-032`'s "already wired" table).
- Per-worktree status: `GetStatus` RPC (`GetStatusResponse { repeated FileStatus files = 1; string branch = 2; }`, `FileStatus { string path = 1; string state = 2; }`) — also wired for real (`git.status`).
- Branch/commit compare primitives: `BranchCompare`/`CommitCompare` RPCs (`gitgateway.proto`) return `changed_files` (int32) and `commits_ahead`, plus a `merge_base` field on `BranchCompareResponse` — the closest thing to a "diff from a shared base SHA" primitive that exists, but it compares exactly two refs at a time, not N worktrees against one base in one call.
- `CommitDiff`/`BranchDiff` RPCs return `FileDiffResponse` for per-file diff detail (single worktree/ref pair at a time).

## What's missing

- **No multi-worktree aggregation endpoint**: nothing in `gitgateway.proto`, `project.proto`, or any wscompat channel accepts a list of worktree IDs and returns a side-by-side comparison in one call — a client must loop over N worktrees itself with no backend support for "these N are being compared as a set."
- **Lines-added/removed metrics**: no diff-related message anywhere in `gitgateway.proto` carries an added/removed line count — `GetDiffResponse` is a single opaque `unified_diff` string, and `BranchCompareResponse`/`CommitCompareResponse` only expose `changed_files` (a count of files, not lines). A client would have to parse the unified diff itself to get lines added/removed; backend-go computes none of it.
- **Test results**: no concept of "test results" exists anywhere in backend-go — confirmed by `grep -rli "test.?result" backend-go/services/*/internal/**/*.go` (excluding `_test.go` files) returning zero matches. There is no service, proto message, or table that stores or returns per-worktree test-run outcomes for the compare view to show.
- **Agent summary output**: same absence — `grep -rli "agent.?summary"` across all non-test `.go` files in `backend-go/services/*/internal/` returns zero matches. Nothing links a worktree to an agent's completion summary anywhere in backend-go.
- **BR-WT-13** (only compare worktrees sharing the same base branch): no validation anywhere — since no aggregation endpoint exists, there is no place to enforce this rule server-side; a client is free to compare arbitrary worktrees regardless of base branch.
- **BR-WT-14** (diff computed from the same base SHA): `merge_base` is returned per-comparison by `BranchCompare`, but nothing forces or verifies that N separate `BranchCompare`/`GetDiff` calls used a consistent base SHA across all N worktrees being compared.
- **BR-WT-15** (no auto-selected winner): trivially true only because no comparison/selection endpoint exists at all — there is nothing that could auto-select a winner, so this rule is vacuously satisfied rather than deliberately enforced.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` — documents that `BranchCompare`/`CommitCompare`/`CommitDiff`/`BranchDiff` exist as real RPCs but notes 28 other `git.*` methods are still unwired; relevant background for what per-worktree primitives this comparison flow could draw on, but does not address the missing aggregation/test-result/agent-summary gaps documented here.

## References

- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `GetDiffResponse`, `GetStatusResponse`, `FileStatus`, `BranchCompareResponse`, `CommitCompareResponse` message definitions
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:221-252` — `git.status`/`git.diff` wiring
- `docs/logic/worktree-management/BL-WT-04-so-sanh-worktree.md`
