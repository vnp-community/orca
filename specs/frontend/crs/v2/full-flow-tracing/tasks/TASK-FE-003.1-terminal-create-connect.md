# TASK-FE-003.1: Đăng ký tracer terminal + instrument `connect()` (tạo PTY session)

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-003 §2.1, §2.2](../solutions/SOL-FE-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001)
**Status:** ✅ Done (2026-08-03) — Same collision pattern as TASK-FE-001.1/002.1: existing `Tracers.terminalCreate/Resize/Destroy/Reattach` (`terminal:*`) are agent-side-PTY-owned — added 4 NEW distinct entries `uiTerminalCreateFlow/ResizeFlow/DestroyFlow/ReconnectFlow` (`ui:terminal.*`) instead. Instrumented `connect()` in `remote-runtime-pty-transport.ts` (real, mounted, live TerminalPane flow — gitnexus_impact LOW risk, 4 downstream symbols, additive-only so behavior preserved); `callRuntimeWithColdStartRetry()` takes an optional `span` param and step()s on retry. `remote-runtime-pty-transport.test.ts` 53/53 pass. Noted: `pty-transport.test.ts` has one PRE-EXISTING unrelated failure (`timeoutMs` expected 15000 vs actual 60000, confirmed present before my changes via git stash) — untouched by me, left as-is per instructions. `pnpm tsc --noEmit` clean.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "createRemoteRuntimePtyTransport"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "createRemoteRuntimePtyTransport", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Không có hàm `createPtyTransport()` factory như TDD-FE-04 mô tả (đã grep xác nhận, không tồn tại verbatim). Điểm chọn transport thật nằm inline trong `PtyConnection` (`pty-connection.ts:3257-3259`): remote (`createRemoteRuntimePtyTransport`, dùng khi có `environmentId`) vs local (`createIpcPtyTransport`, Electron IPC). Task này chỉ xử lý nhánh remote — `connect()` của `createRemoteRuntimePtyTransport()` (`remote-runtime-pty-transport.ts:680-746`), BL-TM-01.

`callRuntimeWithColdStartRetry()` tự retry 1 lần nếu timeout/`relay_starting`/`worker_cold` (cold-start Dev Server có thể mất tới 60s) — đáng `step()` riêng cho lần retry, KHÔNG mở span mới mỗi lần.

## File: `src/shared/trace/tracers.ts` [MODIFY, additive]

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  terminalCreate:    createTracer('ui:terminal.create'),    // BL-TM-01 (split dùng lại tracer này)
  terminalResize:    createTracer('ui:terminal.resize'),    // BL-TM-02 — dùng ở TASK-FE-003.2
  terminalDestroy:   createTracer('ui:terminal.destroy'),   // BL-TM-03 — dùng ở TASK-FE-003.2/003.3
  terminalReconnect: createTracer('ui:terminal.reconnect'), // BL-TM-03 restore — chưa có call site rõ ràng, đặt tên sẵn
} as const
```

> N.B. prefix `ui:`: bắt buộc theo convention chung (xem TASK-FE-001.1/002.1, `00-index.md` mục 1) — 4 tracer trên dùng prefix `ui:` nhất quán với toàn bộ 10 CR.

## File: `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` [MODIFY]

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

export function createRemoteRuntimePtyTransport(
  runtimeEnvironmentId: string,
  opts: IpcPtyTransportOptions = {}
): PtyTransport {
  // ...existing closure setup unchanged...

  return {
    async connect(options) {
      storedCallbacks = options.callbacks
      if (destroyed || !worktreeId) {
        return
      }

      // Why: một span cho toàn bộ connect() — cold-start retry là step(), không attempt riêng.
      const span = Tracers.terminalCreate.start({ worktreeId, providerType: 'remote', environmentId: runtimeEnvironmentId })

      try {
        if (isWebTerminalSurfaceTabId(tabId ?? '')) {
          const result = await attachHostSessionMirror(options)
          if (result) {
            span.ok({ ptyId: result.id, providerType: 'remote', mirrored: true })
          } else {
            span.fail(new Error('attachHostSessionMirror returned undefined'), { worktreeId })
          }
          return result
        }

        // ...existing commandToSend/envToSend/launchConfigToSend/... unchanged...

        span.step('relay-terminal-create', { worktreeId, tabId: tabId ?? '' })
        const created = await callRuntimeWithColdStartRetry<{ terminal: RuntimeTerminalCreate }>(
          'terminal.create',
          {
            worktree: toRuntimeTerminalWorktreeSelector(worktreeId),
            // ...existing command/startupCommandDelivery/env/launchConfig/launchToken/launchAgent...
            tabId, leafId, focus: false, presentation: 'background',
            ...(activate === true ? { activate: true } : {}),
            traceId: span.id
          }
        )
        handle = created.terminal.handle
        if (destroyed) {
          await closeRemoteTerminal(created.terminal.handle)
          span.fail(new Error('connect cancelled after create'), { worktreeId })
          return
        }

        remotePtyId = toRemoteRuntimePtyId(handle, currentRuntimeEnvironmentId)
        connected = true
        desiredViewport = { cols: options.cols ?? 80, rows: options.rows ?? 24 }
        onPtySpawn?.(remotePtyId)

        await subscribeToHandle()
        if (destroyed || !connected || !remotePtyId) {
          span.fail(new Error('connect cancelled after subscribe'), { worktreeId })
          return
        }

        span.ok({ ptyId: remotePtyId, providerType: 'remote' })
        return { id: remotePtyId, replay: '' } satisfies PtyConnectResult
      } catch (error) {
        span.fail(error, { worktreeId, providerType: 'remote' })
        storedCallbacks.onError?.(runtimeTerminalErrorMessage(error))
        return undefined
      }
    },
    // ...rest unchanged (implemented in TASK-FE-003.2)...
  }
}
```

`callRuntimeWithColdStartRetry()` — chỉ thêm 1 dòng `step()` trong nhánh retry, nhận `span` qua closure (không đổi signature public):

```typescript
async function callRuntimeWithColdStartRetry<TResult>(
  method: string,
  params?: unknown,
  span?: TraceSpan
): Promise<TResult> {
  let lastError: unknown
  for (let attempt = 0; attempt <= COLD_START_MAX_RETRIES; attempt++) {
    if (attempt === 0) {
      onColdStartBegin?.()
    } else {
      span?.step('cold-start-retry', { attempt, method })
      onColdStartRetry?.(attempt)
    }
    // ...existing try/catch unchanged...
  }
  onColdStartFailed?.()
  throw lastError
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.terminalCreate/terminalResize/terminalDestroy/terminalReconnect` thêm vào `tracers.ts` đúng tên `ui:terminal.create|resize|destroy|reconnect`
- [ ] `connect()` phát span `ui:terminal.create` bao trọn cold-start retry, `ok()` chứa `ptyId`
- [ ] `traceId: span.id` đính kèm trong params của `terminal.create`
- [ ] Cold-start retry (lỗi `'worker_cold'` ở attempt 0) → `span.step('cold-start-retry', { attempt: 1, method: 'terminal.create' })`
- [ ] Lỗi (relay reject) → `span.fail(error, { worktreeId, providerType: 'remote' })`
- [ ] Không có span/tracer nào được thêm cho `createPtyOutputProcessor()`/OSC 133 scan (BL-TM-04) — xác nhận bằng code review
- [ ] Nhánh local desktop (`createIpcPtyTransport()`) KHÔNG được sửa trong task này — ghi chú rõ là Electron contextBridge IPC, ngoài 6 hàng transport CR-TRACE-000 §3.3
- [ ] Test suite đạt ≥ 5 test case mới: `start()` trước `callRuntimeWithColdStartRetry`, `traceId` trong params, `ok()`/`fail()` đúng field, cold-start retry step
