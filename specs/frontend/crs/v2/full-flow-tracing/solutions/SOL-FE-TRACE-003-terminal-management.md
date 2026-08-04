# SOL-FE-TRACE-003: Terminal Management — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**TDD Ref:** TDD-FE-04 (Terminal Subsystem — xterm.js, PTY Transport, Pane Layout)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts`, `src/shared/trace/tracers.ts`, TracePanel. CR-TRACE-000 (naming convention, `resume` param, quy ước `traceId`).

---

## 1. Điểm khởi tạo trace trong Renderer

### 1.1 Kiến trúc thật: không có `createPtyTransport()` factory như TDD-FE-04 mô tả

TDD-FE-04 §3.2 mô tả một hàm `createPtyTransport({ worktreeId, environmentId, ... })` chọn giữa Local/Remote. Hàm này **không tồn tại verbatim** trong code (đã grep `createPtyTransport\(` trên toàn bộ `terminal-pane/` — không có kết quả ngoài định nghĩa type). Điểm chọn transport thật nằm inline trong `PtyConnection`, tại `src/renderer/src/components/terminal-pane/pty-connection.ts:3257-3259`:

```typescript
const transport = runtimeEnvironmentId
  ? createRemoteRuntimePtyTransport(runtimeEnvironmentId, transportOptions)
  : createIpcPtyTransport(transportOptions)
```

Hai implementation thật:
- **`createRemoteRuntimePtyTransport()`** — `src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts:71` — dùng khi có `environmentId` (web mode / remote runtime). Gọi RPC qua `window.api.runtimeEnvironments.call({ selector, method, params, timeoutMs })` (hàm nội bộ `callRuntime()`, dòng 263-274) — **khác** `callRuntimeRpc()` của `runtime-rpc-client.ts` mà CR-TRACE-001/002 dùng, dù cùng đi WebSocket RPC bên dưới.
- **`createIpcPtyTransport()`** — `src/renderer/src/components/terminal-pane/pty-transport.ts:491` — dùng khi không có `environmentId` (desktop local). Gọi `window.api.pty.*` (Electron `contextBridge` IPC) — **không phải WS RPC**, tương tự gap đã ghi nhận ở SOL-FE-TRACE-001/002 mục "Electron IPC ngoài 6 hàng transport CR-TRACE-000 §3.3".

Đây là instance thứ 3 (sau BL-WT-01 và BL-AG-01) của cùng một pattern kiến trúc: **renderer luôn có 2 nhánh — WS RPC (remote/web) và Electron contextBridge IPC (local/desktop)** — không phải 1 transport chung như flow doc gốc `terminal-management.md` mô tả. CR-TRACE-003 §1 đã ghi nhận điều này ở phía backend (`LocalPtyProvider` vs `SshPtyProvider`); solution này xác nhận renderer cũng rẽ nhánh y hệt, tại đúng điểm chọn transport.

### 1.2 BL-TM-01 — Tạo PTY Session

**Nhánh remote (web/environment):** `connect(options)` — method trả về từ `createRemoteRuntimePtyTransport()`, tại `remote-runtime-pty-transport.ts:680-746`. Gọi:

```typescript
// remote-runtime-pty-transport.ts:699
const created = await callRuntimeWithColdStartRetry<{ terminal: RuntimeTerminalCreate }>('terminal.create', {
  worktree: toRuntimeTerminalWorktreeSelector(worktreeId),
  ...(commandToSend !== undefined ? { command: commandToSend } : {}),
  // ...launchConfig/launchToken/launchAgent/tabId/leafId/presentation...
})
```

`callRuntimeWithColdStartRetry()` (dòng 281-312) tự retry 1 lần nếu timeout/`relay_starting`/`worker_cold` — cold-start Dev Server có thể mất tới 60s (`TERMINAL_CREATE_TIMEOUT_MS`, dòng 46). Đây chính là ví dụ thực tế của CR-TRACE-000 §5 mục 2 ("có khả năng chậm hoặc fail độc lập") — đáng `step()` riêng cho lần retry.

**Nhánh local (desktop):** `connect(options)` của `createIpcPtyTransport()`, tại `pty-transport.ts:681-727`. Gọi `window.api.pty.spawn({ cols, rows, cwd, env, command, ... })` (dòng 695).

### 1.3 BL-TM-02 — Split & Resize

**Split** (mở pane mới trong tab đang mở, dùng khi web client điều khiển host session): `splitWebRuntimeTerminal()` — `src/renderer/src/runtime/web-runtime-session.ts:500-545`. Gọi:

```typescript
// web-runtime-session.ts:523-533
void window.api.runtimeEnvironments.call({
  selector: environmentId,
  method: 'terminal.split',
  params: { terminal: remote.handle, direction, telemetrySource },
  timeoutMs: 15_000
})
```

**Resize — phát hiện lệch tên method so với CR-TRACE-003 §4 BL-TM-02:** CR-TRACE-003 (viết từ phía backend) chỉ đích danh RPC `terminal.resizeForClient` (`src/main/runtime/rpc/methods/terminal.ts:1343`). Method này **có tồn tại** trong `terminal.ts` (đã grep xác nhận, dòng 1343), nhưng **không có call site nào trong renderer gọi nó**. Đường resize thật của remote transport gọi một method khác:

```typescript
// remote-runtime-pty-transport.ts:408-429 — sendViewportUpdate()
void callRuntime('terminal.updateViewport', {
  terminal: targetHandle,
  client: { id: clientId, type: 'desktop' },
  viewport: { cols, rows },
  ...(claim ? { claim: true } : {})
})
```

(Method `terminal.updateViewport` cũng tồn tại thật trong `terminal.ts:1449` — không phải suy đoán.) `resize()`/`claimViewport()` (dòng 903-927) không gọi `sendViewportUpdate()` trực tiếp cho mọi lần kéo pane — chúng đi qua `createRemoteRuntimeViewportBatcher()` (flush mỗi `REMOTE_TERMINAL_VIEWPORT_FLUSH_MS = 33`ms, dòng 41), **trừ khi** `meta?.claim === true` (giành quyền viewport cho client này — gọi `sendViewportUpdate(..., true)` ngay lập tức, không qua batcher).

### 1.4 BL-TM-03 — Destroy & Scrollback: hai đường destroy khác nhau, không phải một

Đã tìm thấy **2 call site destroy độc lập**, ứng với 2 trigger khác nhau — CR-TRACE-003 §4 BL-TM-03 chỉ mô tả 1 luồng chung, cần tách:

**(a) User đóng 1 tab terminal cụ thể:** `closeWebRuntimeTerminal(ptyId)` — `src/renderer/src/runtime/web-runtime-session.ts:622-654`:
```typescript
void window.api.runtimeEnvironments.call({
  selector: environmentId,
  method: 'terminal.close',
  params: { terminal: remote.handle },
  timeoutMs: 15_000
})
```

**(b) Xoá/ngủ cả worktree (nhiều tab cùng lúc):** `shutdownWorktreeTerminals(worktreeId, opts)` — action trong `src/renderer/src/store/slices/terminals.ts:2331`, gọi bằng CR-TRACE-001's `removeWorktree()` (xem SOL-FE-TRACE-001 §2.3) và bởi luồng "sleep worktree". Method RPC khác hẳn — `terminal.stopExact`, có xác minh kết quả:
```typescript
// terminals.ts:2415-2427
stopResult = await callRuntimeRpc<{ stoppedPtyIds?: string[]; livePtyIds?: string[] }>(
  { kind: 'environment', environmentId: runtimeEnvironmentId },
  'terminal.stopExact',
  { worktree: toRuntimeWorktreeSelector(worktreeId), expectedPtyIds: expectedRuntimePtyIds, keepHistory: keepIdentifiers },
  { timeoutMs: 15_000 }
)
// ...verify stoppedPtyIds/livePtyIds/postStopVerified khớp expectedRuntimePtyIds,
// throw 'exact_terminal_stop_mismatch' / 'exact_terminal_stop_unverified' nếu không khớp
```

**Scrollback save — phát hiện khác biệt với CR-TRACE-003 §1 điểm 4:** CR-TRACE-003 (backend) mô tả `writeTerminalScrollbackSnapshotSync` — ghi file **đồng bộ**. Ở renderer, đường save thật cho remote terminal nằm trong `disconnect()` của `createRemoteRuntimePtyTransport()` (`remote-runtime-pty-transport.ts:788-828`):

```typescript
// remote-runtime-pty-transport.ts:799-820 (rút gọn)
if (id && worktreeId && tabId) {
  const stream = multiplexedStream
  if (stream) {
    stream.serializeBuffer?.({ scrollbackRows: 1000 })
      .then(snap => {
        if (snap && worktreeId && tabId) {
          void window.api.terminalSessions?.save?.({
            worktreeId, tabId, leafId: leafId ?? undefined,
            snapshotData: snap.data, snapshotCols: snap.cols, snapshotRows: snap.rows,
          })
        }
      })
      .catch(() => { /* Non-fatal snapshot save */ })
  }
}
```

Đây là **fire-and-forget, bất đồng bộ, qua Electron contextBridge IPC** (`window.api.terminalSessions.save`, "Terminal Session Persistence IPC Bridge — `BUG-FE-TM-003`" theo comment trong `preload/index.ts`) — không phải lời gọi trực tiếp tới hàm sync ở backend mà CR-TRACE-003 mô tả (hàm đó, nếu còn tồn tại, chỉ chạy ở phía Main sau khi nhận IPC này).

**Gap kiến trúc quan trọng cần cảnh báo:** comment trong `pty-transport.ts:670-672` nói rõ *"`shutdownWorktreeTerminals` bypasses the transport layer — it kills PTYs directly via IPC without calling `disconnect()`/`destroy()`"*. Nghĩa là đường (b) — xoá worktree — **không chạy qua `disconnect()` ở trên, do đó không serialize/save scrollback theo cơ chế này**. Route (b) dùng `keepHistory: keepIdentifiers` trong params `terminal.stopExact` để yêu cầu backend tự lo việc giữ lịch sử (nếu `keepIdentifiers === true`, tức sleep chứ không phải xoá hẳn). Với xoá thật (`keepIdentifiers === false`, tức `remove-worktree`), không có scrollback nào được lưu ở renderer — khớp với thiết kế "xoá thì không cần giữ", nhưng **quan trọng để field trong span phản ánh đúng**: `span.step()` của route (b) nên gắn `keepHistory` để TracePanel phân biệt được 2 trường hợp.

### 1.5 BL-TM-04 — OSC 133 / Agent Hook: xác nhận không instrument

`src/shared/terminal-osc133-command-finished.ts` tồn tại thật (đã xác nhận qua `find`), được `createPtyOutputProcessor()` (`pty-transport.ts:132`) dùng nội bộ cho callback `onTitleChange`/`onBell`/`onAgentStatus` trên mỗi PTY chunk — tần suất cao, thuần in-memory. Giữ đúng khuyến nghị CR-TRACE-003 §4 BL-TM-04: **không thêm span/tracer nào** ở đây.

---

## 2. Full Implementation

### 2.1 Tracer mới trong `tracers.ts`

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries unchanged...
  terminalCreate:    createTracer('ui:terminal.create'),    // BL-TM-01 (split dùng lại tracer này)
  terminalResize:    createTracer('ui:terminal.resize'),    // BL-TM-02 (chỉ cho claim-viewport, xem 2.3)
  terminalDestroy:   createTracer('ui:terminal.destroy'),   // BL-TM-03 — cả 2 route (a) và (b)
  terminalReconnect: createTracer('ui:terminal.reconnect'), // BL-TM-03 restore — chưa có call site rõ ràng, đặt tên sẵn
} as const
```

### 2.2 BL-TM-01 — `remote-runtime-pty-transport.ts` (`connect()`)

```typescript
// src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts
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

      // Why: một span cho toàn bộ connect() — cold-start retry là step(), không
      // phải attempt riêng (khác BL-WT-01 vốn retry vì *conflict*, ở đây retry
      // vì *cold start*, ý nghĩa khác nhưng cùng nguyên tắc "1 span/1 operation").
      const span = Tracers.terminalCreate.start({
        worktreeId,
        providerType: 'remote',
        environmentId: runtimeEnvironmentId
      })

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
            tabId,
            leafId,
            focus: false,
            presentation: 'background',
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
    // ...rest unchanged...
  }
}
```

`callRuntimeWithColdStartRetry()` (dòng 281-312) tự thêm `span.step('cold-start-retry', { attempt })` vào lần retry — sửa nội bộ hàm này để nhận `span` qua closure (không đổi signature public, chỉ đọc biến closure có sẵn):

```typescript
// remote-runtime-pty-transport.ts:281-312 — chỉ thêm 1 dòng step() trong nhánh retry
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

### 2.3 BL-TM-02 — Resize: chỉ instrument nhánh `claim`, không instrument batched resize thường

```typescript
// remote-runtime-pty-transport.ts
resize(cols: number, rows: number, meta): boolean {
  if (!connected || !handle) {
    return false
  }
  rememberViewport(cols, rows)
  if (meta?.claim) {
    // Why: claim-viewport là một điểm rẽ nhánh quan trọng (client nào sở hữu
    // input focus của PTY khi có nhiều viewer) — đáng span theo CR-TRACE-000 §5
    // mục 3. Resize thường (không claim) đi qua viewportBatcher ở tần suất
    // cao trong lúc kéo pane — KHÔNG instrument, đúng CR-TRACE-003 §4 nguyên
    // tắc chống over-instrumentation.
    const span = Tracers.terminalResize.start({ worktreeId: worktreeId ?? '', cols, rows, claim: true })
    viewportBatcher.clear()
    sendViewportUpdate(cols, rows, true)
    span.ok({ cols, rows })
    return true
  }
  viewportBatcher.queue(cols, rows)
  return true
},

claimViewport(cols: number, rows: number): boolean {
  if (!connected || !handle) {
    return false
  }
  rememberViewport(cols, rows)
  viewportBatcher.clear()
  const span = Tracers.terminalResize.start({ worktreeId: worktreeId ?? '', cols, rows, claim: true })
  sendViewportUpdate(cols, rows, true)
  span.ok({ cols, rows })
  return true
},
```

> `sendViewportUpdate()` tự nó là fire-and-forget (`.catch(() => {})`, dòng 428) — không await được kết quả thật của `terminal.updateViewport`. `span.ok()` ở đây chỉ xác nhận "đã gửi request", không xác nhận "server đã áp dụng". Đây là hạn chế có sẵn của code hiện tại (không phải điều CR này cần sửa), ghi rõ trong field bằng cách không claim `verified: true` ở bất kỳ đâu.

### 2.4 BL-TM-03 — 2 route destroy

**Route (a) — đóng 1 tab:**

```typescript
// src/renderer/src/runtime/web-runtime-session.ts
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

**Route (b) — teardown cả worktree:**

```typescript
// src/renderer/src/store/slices/terminals.ts
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
  // — nếu không có PTY runtime nào cần dừng, đây thuần là dọn dẹp state in-process,
  // không đáng span theo CR-TRACE-000 §5.
  const span = expectedRuntimePtyIds.length > 0
    ? Tracers.terminalDestroy.start({
        worktreeId,
        route: 'worktree-teardown',
        shutdownReason,
        keepHistory: keepIdentifiers,
        ptyCount: expectedRuntimePtyIds.length
      })
    : undefined

  // ...existing unregisterPtyDataHandlers/disposeParkedTerminalWatchersForPtyIds/
  // sleepingAgentSessionRecords/retainedCompletionEvidence unchanged...

  if (expectedRuntimePtyIds.length > 0) {
    if (!runtimeEnvironmentId) {
      span?.fail(new Error('missing_runtime_for_exact_terminal_stop'), { worktreeId })
      throw new Error('missing_runtime_for_exact_terminal_stop')
    }
    // ...existing suppressedPtyExitIds set()...
    let stopResult: { stoppedPtyIds?: string[]; livePtyIds?: string[]; postStopVerified?: boolean; postStopFailure?: string; remainingLivePtyIds?: string[] }
    try {
      span?.step('relay-terminal-stopExact', { ptyCount: expectedRuntimePtyIds.length })
      stopResult = await callRuntimeRpc<{ stoppedPtyIds?: string[]; livePtyIds?: string[] }>(
        { kind: 'environment', environmentId: runtimeEnvironmentId },
        'terminal.stopExact',
        {
          worktree: toRuntimeWorktreeSelector(worktreeId),
          expectedPtyIds: expectedRuntimePtyIds,
          keepHistory: keepIdentifiers,
          traceId: span?.id
        },
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

### 2.5 Scrollback save — `disconnect()` trong `remote-runtime-pty-transport.ts`

```typescript
disconnect() {
  inputBatcher.flush()
  inputBatcher.clear()
  viewportBatcher.flush()
  outputProcessor.clearAccumulatedState()
  if (!connected && !handle) {
    return
  }
  connected = false
  clearPendingViewportClaim()
  const id = remotePtyId
  if (id && worktreeId && tabId) {
    const stream = multiplexedStream
    if (stream) {
      // Why: không mở span riêng cho save — đây là fire-and-forget best-effort
      // (comment gốc "Non-fatal snapshot save"), nhưng vẫn đáng 1 span nhẹ vì
      // đây là sync-adjacent I/O có thể chậm nếu buffer lớn (CR-TRACE-003 §4
      // BL-TM-03, cùng lý do backend trace writeTerminalScrollbackSnapshotSync).
      const span = Tracers.terminalDestroy.start({ ptyId: id, route: 'scrollback-save', tabId })
      stream.serializeBuffer?.({ scrollbackRows: 1000 })
        .then(snap => {
          if (snap && worktreeId && tabId) {
            void window.api.terminalSessions?.save?.({
              worktreeId, tabId, leafId: leafId ?? undefined,
              snapshotData: snap.data, snapshotCols: snap.cols, snapshotRows: snap.rows,
            }).then(() => span.ok({ ptyId: id, bufferBytes: snap.data.length }))
              .catch((err) => span.fail(err, { ptyId: id }))
          } else {
            span.ok({ ptyId: id, skipped: true })
          }
        })
        .catch((err) => span.fail(err, { ptyId: id }))
    }
  }
  closeMultiplexedStream()
  handle = null
  remotePtyId = null
  storedCallbacks.onDisconnect?.()
  if (id) {
    onPtyExit?.(id)
  }
}
```

> `route: 'scrollback-save'` dùng cùng tracer `terminalDestroy` nhưng field `route` khác 2 route ở mục 2.4 — cho phép TracePanel lọc riêng "chỉ xem các lần save scrollback" mà không lẫn với việc đóng PTY. Đây là 1 trong 3 span độc lập có thể xảy ra khi 1 tab bị đóng thường (route (a) đóng PTY + route "scrollback-save" chạy song song, không lồng nhau — chúng không share `id`/`resume` vì chạy độc lập, không có quan hệ cha-con theo quy ước CR-TRACE-001 §4).

---

## 3. Test Plan (Vitest)

```
src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.test.ts   (file đã tồn tại — thêm test case)
├── connect() gọi Tracers.terminalCreate.start({ worktreeId, providerType: 'remote', environmentId }) trước callRuntimeWithColdStartRetry
├── connect() truyền traceId: span.id vào params của 'terminal.create'
├── connect() thành công → span.ok({ ptyId, providerType: 'remote' })
├── connect() lỗi (relay reject) → span.fail(error, { worktreeId, providerType: 'remote' })
├── cold-start retry (lỗi 'worker_cold' ở attempt 0) → span.step('cold-start-retry', { attempt: 1, method: 'terminal.create' })
├── resize({ claim: true }) mở span ui:terminal.resize, resize thường (claim: false/undefined) KHÔNG mở span nào
├── claimViewport() luôn mở span ui:terminal.resize
├── disconnect() với multiplexedStream tồn tại → mở span ui:terminal.destroy route:'scrollback-save', ok() khi save thành công
└── disconnect() không có worktreeId/tabId → không mở span save (nhánh skip)

src/renderer/src/runtime/web-runtime-session.test.ts   (file đã tồn tại — thêm test case)
├── closeWebRuntimeTerminal() mở span ui:terminal.destroy route:'single-tab-close', truyền traceId vào params
├── closeWebRuntimeTerminal() thành công → span.ok({ ptyId })
└── closeWebRuntimeTerminal() lỗi → span.fail(error, { ptyId })

src/renderer/src/store/slices/terminals.test.ts   (file đã tồn tại — thêm test case)
├── shutdownWorktreeTerminals() với expectedRuntimePtyIds rỗng → KHÔNG mở span (chỉ dọn state in-process)
├── shutdownWorktreeTerminals() với expectedRuntimePtyIds > 0 → mở span ui:terminal.destroy route:'worktree-teardown', field keepHistory đúng theo keepIdentifiers
├── stopResult mismatch (stoppedPtyIds lệch expected) → span.fail('exact_terminal_stop_mismatch')
├── postStopVerified !== true → span.fail(postStopFailure)
└── thành công → span.ok({ worktreeId, stoppedCount })
```

**Mock pattern:** tương tự SOL-FE-TRACE-001/002 — `vi.spyOn(Tracers.terminalCreate, 'start')` trả về span giả (`{ id, step: vi.fn(), ok: vi.fn(), fail: vi.fn() }`), assert cả tham số field truyền vào `start()`/`step()`/`ok()`/`fail()` lẫn việc `traceId: span.id` xuất hiện đúng vị trí trong params RPC — theo pattern mock `IRpcClient`/`callRuntimeRpc` đã mô tả ở TDD-FE-03.

**Target:** ≥ 18 test case mới (9 cho `remote-runtime-pty-transport.test.ts`, 3 cho `web-runtime-session.test.ts`, 5 cho `terminals.test.ts`, cộng dồn với test suite có sẵn của các file này).

---

## 4. Acceptance Criteria

- [ ] `Tracers.terminalCreate/terminalResize/terminalDestroy/terminalReconnect` được thêm vào `tracers.ts` đúng tên `ui:terminal.create|resize|destroy|reconnect`
- [ ] `connect()` của `createRemoteRuntimePtyTransport()` (`remote-runtime-pty-transport.ts:680`) phát span `ui:terminal.create` bao trọn cold-start retry, `ok()` chứa `ptyId`
- [ ] `resize()`/`claimViewport()` chỉ mở span `ui:terminal.resize` cho nhánh `claim === true` — resize thường qua `viewportBatcher` (33ms) KHÔNG có span, xác nhận bằng code review + test đếm số lần `Tracers.terminalResize.start` được gọi khi giả lập một chuỗi resize kéo pane
- [ ] `closeWebRuntimeTerminal()` (route a) và `shutdownWorktreeTerminals()` (route b) dùng CHUNG tracer `ui:terminal.destroy` nhưng field `route` khác nhau (`'single-tab-close'` vs `'worktree-teardown'`), cho phép phân biệt trong TracePanel
- [ ] `shutdownWorktreeTerminals()` không mở span khi `expectedRuntimePtyIds.length === 0` (chỉ dọn dẹp state in-process, đúng CR-TRACE-000 §5)
- [ ] Scrollback save trong `disconnect()` (`remote-runtime-pty-transport.ts:799-820`) có span riêng route `'scrollback-save'`, field `bufferBytes` khi có, không throw nếu `window.api.terminalSessions.save` reject (giữ nguyên hành vi "Non-fatal" hiện tại)
- [ ] Ghi rõ trong PR/commit rằng RPC method resize thật là `terminal.updateViewport`, KHÔNG phải `terminal.resizeForClient` như CR-TRACE-003 (backend) đặt tên ban đầu — cần đồng bộ lại với companion backend CR trước khi backend thêm tracer cho method sai
- [ ] Không có span/tracer nào được thêm cho `createPtyOutputProcessor()`/OSC 133 scan (BL-TM-04) — xác nhận bằng code review
- [ ] Nhánh local desktop (`createIpcPtyTransport()`, `window.api.pty.spawn/resize/kill`) được ghi chú rõ là Electron contextBridge IPC, ngoài 6 hàng transport CR-TRACE-000 §3.3 — nhất quán với gap đã nêu ở SOL-FE-TRACE-001/002, không tự ý thêm `traceId` phía đó trong CR này (để tránh 3 CR tự ý mở rộng quy ước theo 3 cách khác nhau — nên xử lý tập trung ở một CR-TRACE-000 addendum riêng)
