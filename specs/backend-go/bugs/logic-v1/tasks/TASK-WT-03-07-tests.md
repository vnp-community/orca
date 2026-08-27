# TASK-WT-03-07: Tests for delete-safety checks, agent-kill, and the new channels

**From Solution:** SOL-WT-03
**Priority:** P1
**Service:** `git-gateway-service` + `api-gateway`
**File:** `backend-go/services/git-gateway-service/internal/usecase/remove_worktree_test.go`
**Depends on:** TASK-WT-03-04, TASK-WT-03-05, TASK-WT-03-06
**Status:** `[ ]` TODO

---

## Context

Regression coverage per [SOL-WT-03](../solutions/SOL-WT-03-xoa-worktree.md)'s Test plan — `_UncommittedChanges_ForceFalse_RejectsBeforeGitCall` is the core regression guard against today's bypass (this bug's own finding: `force=true` fully bypasses the only existing check).

## Changes to make

`backend-go/services/git-gateway-service/internal/usecase/check_worktree_delete_safety_test.go` (new):
- `TestCheckWorktreeDeleteSafety_CountsUncommittedAndUntrackedSeparately` — fake `GetStatus` with mixed `FileStatus.State` values.
- `TestCheckWorktreeDeleteSafety_NoActiveConnection_AgentRunningFalse_NoTerminalCall` — fake `ConnectionResolver` returns `Connected: false`; assert the fake `TerminalSessionLister.ListSessions` was never called.
- `TestCheckWorktreeDeleteSafety_ActiveSessionInWorktree_ReportsPtyID` — fake `ListSessions` returns one session with a matching-`cwd` prefix and one with a non-matching `cwd`; assert only the matching one appears in `ActivePtyIDs`.
- `TestCheckWorktreeDeleteSafety_SafeToDelete_TrueOnlyWhenAllCountsZero`.

Update `backend-go/services/git-gateway-service/internal/usecase/remove_worktree_test.go` (existing file, now needs the new `terminals TerminalSessionLister` fake and `RemoveWorktreeInput` shape):
- `TestRemoveWorktree_UncommittedChanges_ForceFalse_RejectsBeforeGitCall` — assert the fake executor's `RemoveWorktree` is never called (regression guard against the current bypass).
- `TestRemoveWorktree_UncommittedChanges_ForceTrue_ProceedsToGitCall`.
- `TestRemoveWorktree_AgentRunning_StopAgentsFalse_RejectsBeforeGitCall`.
- `TestRemoveWorktree_AgentRunning_StopAgentsTrue_KillsSessionsThenRemoves` — assert `Kill` called once per active session and `RemoveWorktree` called after.
- `TestRemoveWorktree_KillFails_StillProceedsWithRemoval_BestEffort`.
- Update every existing happy-path/bookkeeping-stale test for the new `RemoveWorktreeInput`/`RemoveWorktreeResult` shapes.

`backend-go/services/git-gateway-service/internal/adapter/infraclient/terminal_sessions_test.go` (new) — contract test against a fake `infrafleetv1.InfraFleetServiceClient`: `ListSessions` maps `TerminalSession.cwd`/`pty_id` correctly; `Kill` forwards `pty_id`.

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go` — add:
- `TestWorktreeCheckDeleteSafety_HappyPath`.
- `TestWorktreeRm_StopAgentsThreadsThroughToGRPCRequest` — assert the fake `gitClient.RemoveWorktree` call's request has `StopAgents` set from the channel arg.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/git-gateway-service/internal/usecase/... ./services/git-gateway-service/internal/adapter/infraclient/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run "WorktreeCheckDeleteSafety|WorktreeRm"
```

Expected: all cases pass.
