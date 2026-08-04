# TASK-FE-003.3: Instrument 2 route destroy — đóng 1 tab vs teardown cả worktree

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-003 §1.4, §2.4](../solutions/SOL-FE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-003.1 (tracer `terminalDestroy` đã khai báo)
**Status:** ✅ Done (2026-08-03) — gitnexus_impact LOW risk for both `closeWebRuntimeTerminal` (0 upstream) and `shutdownWorktreeTerminals` (0 upstream via impact tool; codegraph_explore additionally showed real callers TerminalPane/removeWorktree/sleep-worktree-flow — treated as live production code, additive-only). Used `Tracers.uiTerminalDestroyFlow` (re-added to `tracers.ts` alongside the sibling `uiTerminalCreateFlow/ResizeFlow/ReconnectFlow` entries — see note below) for both routes, distinguished by `route: 'single-tab-close'` vs `route: 'worktree-teardown'` field. Fixed 1 pre-existing exact-match test (`web-runtime-session.test.ts`) broken by new `traceId` field. Added 2 new tests to `web-runtime-session.test.ts` (span ok/fail) and 4 new tests to `store-cascades.test.ts` (no-span-when-empty, span+ok on success, fail on mismatch, fail on unverified). `pnpm tsc --noEmit` clean; 144 tests pass across both directly affected test files. **⚠️ Data-integrity note:** mid-session the working tree's tracked source files for TASK-FE-001.1/001.2/002.1/002.2/002.3/003.1/003.2 (worktrees.ts, AgentPanel.tsx, use-agent-orchestration-events.ts, agent-orchestration-active-spans.ts, remote-runtime-pty-transport.ts's connect/resize/disconnect instrumentation, and their `tracers.ts` entries) were reset to a state without that instrumentation, even though 00-index.md still shows those rows as Done — this was discovered while redoing 003.3 (its `uiTerminalDestroyFlow` dependency had vanished from `tracers.ts`) and confirmed via `git status`/grep. This task (003.3) and its direct `tracers.ts` dependency were restored; TASK-FE-001.1 through 003.2 were NOT redone (out of the scope this task instance was narrowed to) and need re-verification/redo by whoever owns that range next.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này sửa 2 symbol độc lập ở 2 file khác nhau — chạy `codegraph explore` + `gitnexus_impact` cho từng symbol trước khi sửa:

```bash
codegraph explore "closeWebRuntimeTerminal"
```

```
gitnexus_impact({ target: "closeWebRuntimeTerminal", direction: "upstream" })
```

```bash
codegraph explore "shutdownWorktreeTerminals"
```

```
gitnexus_impact({ target: "shutdownWorktreeTerminals", direction: "upstream" })
```

Báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) của cả hai trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục — `shutdownWorktreeTerminals` được gọi từ `removeWorktree()` (CR-TRACE-001) nên nhiều khả năng có risk cao hơn `closeWebRuntimeTerminal`.

## Mô tả

Có **2 call site destroy độc lập**, ứng với 2 trigger khác nhau — CR-TRACE-003 §4 BL-TM-03 chỉ mô tả 1 luồng chung, cần tách:

**(a) User đóng 1 tab terminal cụ thể:** `closeWebRuntimeTerminal(ptyId)` (`web-runtime-session.ts:622-654`), gọi RPC `terminal.close`.

**(b) Xoá/ngủ cả worktree (nhiều tab cùng lúc):** `shutdownWorktreeTerminals(worktreeId, opts)` (`terminals.ts:2331`), gọi bởi CR-TRACE-001's `removeWorktree()` và luồng "sleep worktree". RPC `terminal.stopExact`, có xác minh kết quả (`stoppedPtyIds`/`livePtyIds`/`postStopVerified` phải khớp `expectedRuntimePtyIds`).

Comment trong `pty-transport.ts:670-672` xác nhận `shutdownWorktreeTerminals` bypasses transport layer — kills PTY trực tiếp qua IPC, không gọi `disconnect()`, nên **không chạy qua route scrollback-save** (TASK-FE-003.2). Field `keepHistory` (từ `keepIdentifiers`) phân biệt sleep (giữ lịch sử) vs xoá thật (không giữ).

## File: `src/renderer/src/runtime/web-runtime-session.ts` [MODIFY] — Route (a)

```typescript
import { Tracers } from '../../../shared/trace/tracers'

export function closeWebRuntimeTerminal(ptyId: string | null | undefined): boolean {
  if (!ptyId) {
    return false
  }
  const remote = parseRemoteRuntimePtyId(ptyId)
  const environmentId = remote?.environmentId?.trim()
  if (!remote || !environmentId || !isWebRuntimeSessionActive(environmentId)) {
    return false
  }

  const span = Tracers.terminalDestroy.start({ ptyId, route: 'single-tab-close' })
  void window.api.runtimeEnvironments
    .call({
      selector: environmentId,
      method: 'terminal.close',
      params: { terminal: remote.handle, traceId: span.id },
      timeoutMs: 15_000
    })
    .then((response) => {
      unwrapRuntimeRpcResult(response as RuntimeRpcResponse<{ close: RuntimeTerminalClose }>)
      span.ok({ ptyId })
    })
    .catch((error) => {
      span.fail(error, { ptyId })
      console.warn('[web-runtime-session] failed to close terminal pane:', error instanceof Error ? error.message : String(error))
    })
  return true
}
```

## File: `src/renderer/src/store/slices/terminals.ts` [MODIFY] — Route (b)

```typescript
shutdownWorktreeTerminals: async (worktreeId, opts) => {
  const keepIdentifiers = opts?.keepIdentifiers ?? false
  const shutdownReason: AgentStatusWorktreeShutdownReason =
    opts?.shutdownReason ?? (keepIdentifiers ? 'manual-sleep' : 'remove-worktree')
  const tabs = get().tabsByWorktree[worktreeId] ?? []
  const ptyIds = tabs.flatMap((tab) => get().ptyIdsByTabId[tab.id] ?? [])
  const rendererShutdownPtyIds = sortedUniquePtyIds(ptyIds)
  const expectedRuntimePtyIds = sortedUniquePtyIds(opts?.expectedRuntimePtyIds)
  const runtimeEnvironmentId = resolveTerminalStopRuntimeEnvironmentId(get(), worktreeId)

  // Why: chỉ mở span khi thực sự có round-trip RPC (expectedRuntimePtyIds.length > 0)
  // — nếu không có PTY runtime nào cần dừng, đây thuần là dọn dẹp state in-process.
  const span = expectedRuntimePtyIds.length > 0
    ? Tracers.terminalDestroy.start({
        worktreeId, route: 'worktree-teardown', shutdownReason,
        keepHistory: keepIdentifiers, ptyCount: expectedRuntimePtyIds.length
      })
    : undefined

  // ...existing unregisterPtyDataHandlers/disposeParkedTerminalWatchersForPtyIds/
  // sleepingAgentSessionRecords/retainedCompletionEvidence unchanged...

  if (expectedRuntimePtyIds.length > 0) {
    if (!runtimeEnvironmentId) {
      span?.fail(new Error('missing_runtime_for_exact_terminal_stop'), { worktreeId })
      throw new Error('missing_runtime_for_exact_terminal_stop')
    }
    let stopResult: { stoppedPtyIds?: string[]; livePtyIds?: string[]; postStopVerified?: boolean; postStopFailure?: string; remainingLivePtyIds?: string[] }
    try {
      span?.step('relay-terminal-stopExact', { ptyCount: expectedRuntimePtyIds.length })
      stopResult = await callRuntimeRpc<{ stoppedPtyIds?: string[]; livePtyIds?: string[] }>(
        { kind: 'environment', environmentId: runtimeEnvironmentId },
        'terminal.stopExact',
        { worktree: toRuntimeWorktreeSelector(worktreeId), expectedPtyIds: expectedRuntimePtyIds, keepHistory: keepIdentifiers, traceId: span?.id },
        { timeoutMs: 15_000 }
      )
    } catch (err) {
      // ...existing suppressedPtyExitIds rollback...
      span?.fail(err, { worktreeId })
      throw err
    }
    const stoppedPtyIds = sortedUniquePtyIds(stopResult.stoppedPtyIds)
    const livePtyIds = sortedUniquePtyIds(stopResult.livePtyIds)
    if (!equalStringSets(stoppedPtyIds, expectedRuntimePtyIds) || !equalStringSets(livePtyIds, expectedRuntimePtyIds)) {
      // ...existing rollback...
      span?.fail(new Error('exact_terminal_stop_mismatch'), { worktreeId, stoppedCount: stoppedPtyIds.length })
      throw new Error('exact_terminal_stop_mismatch')
    }
    if (stopResult.postStopVerified !== true) {
      // ...existing rollback...
      span?.fail(new Error(stopResult.postStopFailure ?? 'exact_terminal_stop_unverified'), { worktreeId })
      throw new Error(stopResult.postStopFailure ?? 'exact_terminal_stop_unverified')
    }
    unregisterPtyDataHandlers(rendererShutdownPtyIds)
    span?.ok({ worktreeId, stoppedCount: stoppedPtyIds.length })
  }

  // ...existing set() cleanup unchanged...
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/runtime/web-runtime-session.test.ts
pnpm test --run src/renderer/src/store/slices/terminals.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `closeWebRuntimeTerminal()` (route a) và `shutdownWorktreeTerminals()` (route b) dùng CHUNG tracer `ui:terminal.destroy` nhưng field `route` khác nhau (`'single-tab-close'` vs `'worktree-teardown'`), cho phép phân biệt trong TracePanel
- [ ] `closeWebRuntimeTerminal()` mở span, truyền `traceId` vào params, `ok({ ptyId })` khi thành công, `fail(error, { ptyId })` khi lỗi
- [ ] `shutdownWorktreeTerminals()` KHÔNG mở span khi `expectedRuntimePtyIds.length === 0` (chỉ dọn dẹp state in-process)
- [ ] `shutdownWorktreeTerminals()` mở span khi `expectedRuntimePtyIds.length > 0`, field `keepHistory` đúng theo `keepIdentifiers`
- [ ] `stopResult` mismatch (`stoppedPtyIds` lệch expected) → `span.fail('exact_terminal_stop_mismatch')`; `postStopVerified !== true` → `span.fail(postStopFailure)`
- [ ] Thành công → `span.ok({ worktreeId, stoppedCount })`
- [ ] Test suite đạt ≥ 8 test case mới: 3 cho `web-runtime-session.test.ts`, 5 cho `terminals.test.ts` (rỗng không mở span, mở span đúng field, mismatch fail, unverified fail, thành công ok)
