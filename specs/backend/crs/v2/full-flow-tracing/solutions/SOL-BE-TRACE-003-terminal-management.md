# SOL-BE-TRACE-003: Terminal Management — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**TDD Ref:** TDD-03 (PTY Daemon Layer — local PTY lifecycle), TDD-08 (Agent Orchestration — cross-ref cho nhánh AI-agent-PTY, xem 1.4)
**Date:** 2026-08-02
**Status:** Proposed
**Test Targets:** ≥ 18 tests (xem mục 3)
**Strategy:** Additive-only — instrument tại RPC boundary (`terminal.ts`) và tại 2 provider thật (`LocalPtyProvider`, `SshPtyProvider`); KHÔNG động tới nhánh AI-agent-PTY (đã thuộc SOL-BE-TRACE-002)

---

## 1. Phân tích phạm vi (Backend-side only)

### 1.1 Lệch giữa TDD-03 và code thật — có 3 con đường PTY, TDD chỉ mô tả 1

TDD-03 (PTY Daemon Layer) mô tả **Daemon Process** (`src/main/daemon/`) — Unix socket, NDJSON, `pty.create`/`pty.write`/`pty.resize`/`pty.kill` — đây là cơ chế **local PTY thật sự dùng cho desktop mode** (`DaemonPtyAdapter` → `DaemonServer` → `PtySubprocess`). Đối chiếu với CR-TRACE-003 §1 và grep trực tiếp source, xác nhận: TDD-03 mô tả đúng cơ chế bên dưới `LocalPtyProvider`, nhưng **không phải là con đường duy nhất**. Có 3 con đường PTY độc lập trong code:

| # | Con đường | File | Dùng cho |
|---|---|---|---|
| 1 | `LocalPtyProvider` → `DaemonPtyAdapter` (TDD-03) | `src/main/providers/local-pty-provider.ts:312`, `spawn()` dòng 330 | Terminal người dùng mở tay, host local |
| 2 | `SshPtyProvider` → `SshChannelMultiplexer`/`SshRelaySession` (TDD-05) | `src/main/providers/ssh-pty-provider.ts:35`, `spawn()` dòng 100 | Terminal người dùng mở tay, host SSH |
| 3 | `agent-rpc-dispatch.ts` case `pty.create/resize/destroy/scrollback` (TDD-08/CR-TRACE-002) | `src/relay/agent-rpc-dispatch.ts:630-706` | PTY riêng cho AI agent trên Dev Server (Project/Web mode) |

TDD-03 chỉ tài liệu hoá con đường #1. CR-TRACE-003 đã xác nhận đúng: `WorkspaceContextManager` (component trong flow doc gốc) không tồn tại — `terminal.create` RPC gọi thẳng `OrcaRuntimeService.createTerminal()` (`orca-runtime.ts:17470`), hàm này tự chọn provider #1 hay #2 tuỳ `resolveExecutionHost()`. SOL này chỉ đề cập tới con đường #1 và #2 (backend/gateway phía Orca Server thật sự sở hữu); con đường #3 thuộc CR-TRACE-002 (xem 1.4).

### 1.2 Phát hiện bổ sung so với CR-TRACE-003 — cả 2 provider đều có method `spawn()` (đã verify)

CR-TRACE-003 §4 BL-TM-01 ghi "method spawn cụ thể chưa xác định tên hàm chính xác, cần xác nhận thêm khi implement". Đã verify bằng Read trực tiếp:

- `LocalPtyProvider.spawn(args: PtySpawnOptions): Promise<PtySpawnResult>` — `local-pty-provider.ts:330`
- `SshPtyProvider.spawn(opts: PtySpawnOptions): Promise<PtySpawnResult>` — `ssh-pty-provider.ts:100`

Cả 2 cùng implement interface `IPtyProvider` — cùng tên method `spawn()`, cùng shape tham số/kết quả (`PtySpawnOptions`/`PtySpawnResult`). Đây là điểm instrument tốt hơn so với việc chỉ wrap ở RPC handler, vì `spawn()` là nơi phân biệt rõ ràng Local vs SSH ngay tại chữ ký hàm, không cần suy luận qua field runtime nội bộ.

### 1.3 Phát hiện bổ sung — scrollback save/restore không đi qua RPC nào cả, mà qua Electron IPC

CR-TRACE-003 §4 BL-TM-03 ghi "chưa xác định file cụ thể gọi `writeTerminalScrollbackSnapshotSync`". Đã truy ngược bằng grep toàn bộ `src/`:

- `writeTerminalScrollbackSnapshotSync()` (`terminal-scrollback-snapshots.ts:98`) **không có caller trực tiếp nào ngoài `migrateWorkspaceSessionTerminalScrollbackSnapshots()`** (cùng file, dòng 173, gọi write ở dòng 187) — đây là một **batch migration pass**, không phải "save khi 1 terminal cụ thể bị destroy" như CR-TRACE-003 giả định. Hàm này duyệt toàn bộ `session.terminalLayoutsByTabId`, với mỗi tab/leaf có `buffersByLeafId` (buffer scrollback in-memory chưa được ghi file) thì gọi `writeTerminalScrollbackSnapshotSync()` và thay bằng `scrollbackRefsByLeafId` (con trỏ file).
- `migrateWorkspaceSessionTerminalScrollbackSnapshots()` được gọi từ **2 nơi trong `src/main/persistence.ts`**: `Store.load()` (khởi động app, dòng ~3541) và `Store.setWorkspaceSession()` (mỗi lần workspace session được ghi đè, dòng ~5895).
- `Store.setWorkspaceSession()`/`Store.patchWorkspaceSession()` **không được gọi từ bất kỳ WS RPC method nào** (đã grep toàn bộ `src/main/runtime/rpc/methods/` — không có kết quả) — chỉ được gọi từ **Electron IPC handler** `src/main/ipc/session.ts`: `ipcMain.handle('session:set', ...)`, `ipcMain.on('session:set-sync', ...)` (sync variant cho `beforeunload`), `ipcMain.handle('session:patch', ...)`.
- Restore (đọc) cũng vậy: `Store.readTerminalScrollbackSnapshot(ref)` (`persistence.ts:5720`, wrapper gọi `readTerminalScrollbackSnapshotSync`) chỉ có 1 caller thật ngoài test: `ipcMain.on('session:read-terminal-scrollback-sync', ...)` (`src/main/ipc/session.ts:29-35`).

**Hệ quả cho tracing:** đây là kênh **Electron `contextBridge`/`ipcMain` (renderer ↔ main, cùng máy, cùng process tree)** — không nằm trong bất kỳ 6 dòng transport nào của CR-TRACE-000 §3.3 (bảng đó liệt kê WS RPC, `relay.call()`, Agent WS JSON-RPC, HTTP/WS CLI, Mobile WS, SSH exec — không có "Electron IPC nội bộ"). Vì đây là lời gọi đồng bộ trong cùng process (Electron main), xử lý như các bước in-process khác trong CR-TRACE-000 (không cần wire `traceId` — chỉ 1 process duy nhất tham gia). Việc `terminal-management.md` (flow doc gốc) mô tả `relay.call('pty.scrollback')` → gzip → DB `orca_terminal_scrollback_snapshots` là **hoàn toàn không khớp thực tế** — không có bảng đó, không có gzip, không có relay call nào; chỉ là sync file I/O cục bộ.
- Đường tương đương cho **Web Server mode** (non-Electron, không có `ipcMain`) **chưa xác định — cần điều tra thêm khi triển khai**: `session.tabs.*` RPC methods (`session-tabs.ts`) tồn tại nhưng không thấy gọi `setWorkspaceSession`/`migrateWorkspaceSessionTerminalScrollbackSnapshots` qua grep trực tiếp — có thể web mode dùng cơ chế lưu trữ khác cho scrollback, hoặc chưa hỗ trợ đầy đủ. Solution này chỉ instrument đường Electron IPC đã verify; đường Web Server mode cần một CR bổ sung sau khi xác nhận kiến trúc thật.

### 1.4 Gap table

| Sub-flow | RPC method / File | Hiện trạng | Hành động backend-side |
|---|---|---|---|
| BL-TM-01 Create | `terminal.create` (`terminal.ts:1284`) → `runtime.createTerminal()` (`orca-runtime.ts:17470`) → `LocalPtyProvider.spawn()`/`SshPtyProvider.spawn()` | Không có instrumentation | Wrap RPC handler bằng `Tracers.terminalCreate`; `span.step('provider-spawn', { providerType })` — chọn đo tại RPC boundary (không đủ cơ sở để chèn `span` object xuyên vào `createTerminal()` nội bộ 26K dòng mà không rủi ro side-effect, xem 2.3) |
| BL-TM-02 Split | `terminal.split` (`terminal.ts:1316`) → `runtime.splitTerminal()` | Không có instrumentation | Dùng lại `Tracers.terminalCreate` (không tạo tracer riêng, theo CR-TRACE-003 §3) |
| BL-TM-02 Resize | `terminal.resizeForClient` (`terminal.ts:1343`) → `runtime.resizeForClient()` | Không có instrumentation | `Tracers.terminalResize` riêng |
| BL-TM-03 Save (scrollback) | `migrateWorkspaceSessionTerminalScrollbackSnapshots()` (`terminal-scrollback-snapshots.ts:173`), gọi từ `Store.setWorkspaceSession()` (`persistence.ts:~5895`), trigger bởi Electron IPC `session:set`/`session:set-sync` (`ipc/session.ts`) | Không có instrumentation | Wrap `migrateWorkspaceSessionTerminalScrollbackSnapshots()` bằng `Tracers.terminalDestroy`, CHỈ khi có buffer thật cần ghi (guard chống over-instrumentation — method này được gọi rất thường xuyên) |
| BL-TM-03 Restore (scrollback) | `Store.readTerminalScrollbackSnapshot()` (`persistence.ts:5720`), trigger bởi Electron IPC `session:read-terminal-scrollback-sync` | Không có instrumentation | Wrap bằng `Tracers.terminalReconnect` tại IPC handler (`ipc/session.ts`) |
| BL-TM-04 OSC 133 | `src/shared/terminal-osc133-command-finished.ts` | In-memory, tần suất cao | KHÔNG instrument (theo CR-TRACE-000 §5 và CR-TRACE-003 §4 — xác nhận lại) |

### 1.5 Ngoài phạm vi (out of scope)

- Nhánh AI-agent-PTY (`agent-rpc-dispatch.ts` case `pty.create/resize/destroy/scrollback`, dùng khi `ProfileAwareAgentSpawner`/`agent.exec` cần PTY riêng trên Dev Server) — đã thuộc **SOL-BE-TRACE-002** (`agentOrch:spawn` span bao trọn lời gọi `agent.exec`, trong đó PTY creation là chi tiết implementation phía Dev Server). Không lặp lại instrumentation ở đây.
- Bất kỳ điều gì xảy ra bên trong Daemon process (`src/main/daemon/*`) sau khi `LocalPtyProvider` gọi `DaemonPtyAdapter` — đây là ranh giới Unix socket nội bộ giữa Electron main và Daemon process, không phải ranh giới CR-TRACE-000 liệt kê (không phải cross-host); solution này dừng lại ở `LocalPtyProvider.spawn()`, không xuyên vào `DaemonPtyAdapter`/Daemon process.
- Con đường Web Server mode cho scrollback (xem 1.3, mục cuối) — chưa đủ cơ sở để instrument.

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries (worktree:*, agentOrch:*, devServer:*, ...) unchanged...

  // ─── CR-TRACE-003: Terminal Management (BL-TM-01→04) ───────────────────────
  /** terminal.create + terminal.split — BL-TM-01/02 */
  terminalCreate:    createTracer('terminal:create'),
  /** terminal.resizeForClient — BL-TM-02 */
  terminalResize:    createTracer('terminal:resize'),
  /** scrollback save (migrateWorkspaceSessionTerminalScrollbackSnapshots write path) — BL-TM-03 */
  terminalDestroy:   createTracer('terminal:destroy'),
  /** scrollback restore (readTerminalScrollbackSnapshotSync) — BL-TM-03 */
  terminalReconnect: createTracer('terminal:reconnect'),
} as const
```

### 2.2 `src/main/runtime/rpc/methods/terminal.ts` — instrument `terminal.create` / `.split` / `.resizeForClient`

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

// TerminalCreateParams — thêm field:
const TerminalCreateParams = z.object({
  // ...existing fields (worktree, command, env, launchConfig, ...) unchanged...
  traceId: OptionalString, // [NEW CR-TRACE-003]
})

// ...

defineMethod({
  name: 'terminal.create',
  params: TerminalCreateParams,
  handler: async (params, ctx) => {
    if (!ctx.userId) {
      throw new Error('UNAUTHORIZED: terminal.create requires an authenticated session. Please log in.')
    }
    const span = Tracers.terminalCreate.start(
      { worktree: params.worktree ?? '' },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const { runtime } = ctx
      const terminal = await runtime.createTerminal(params.worktree, {
        // ...existing options (command, env, launchConfig, title, focus, ...) unchanged...
      })
      // Why: createTerminal() tự chọn Local/SSH provider nội bộ (resolveExecutionHost);
      // providerType không lộ ra qua kết quả trả về hôm nay — dùng field terminal.ptyId
      // prefix hoặc runtime.resolveExecutionHost(params.worktree) để suy ra nếu cần chi tiết
      // hơn khi implement thật (xem 2.3 cho instrumentation trực tiếp tại provider).
      span.ok({ ptyId: terminal.ptyId })
      return { terminal }
    } catch (err) {
      span.fail(err, { worktree: params.worktree ?? '' })
      throw err
    }
  }
}),

defineMethod({
  name: 'terminal.split',
  params: TerminalSplit,
  handler: async (params, { runtime }) => {
    const span = Tracers.terminalCreate.start({ terminal: params.terminal, direction: params.direction ?? '' })
    try {
      const split = await runtime.splitTerminal(params.terminal, {
        direction: params.direction,
        command: params.command,
        env: params.env,
        telemetrySource: params.telemetrySource
      })
      span.ok({ ptyId: split.ptyId })
      return { split }
    } catch (err) {
      span.fail(err, { terminal: params.terminal })
      throw err
    }
  }
}),

// ...

defineMethod({
  name: 'terminal.resizeForClient',
  params: TerminalResizeForClient,
  handler: async (params, { runtime }) => {
    const span = Tracers.terminalResize.start({ terminal: params.terminal, mode: params.mode })
    const leaf = runtime.resolveLiveLeafForHandle(params.terminal)
    if (!leaf?.ptyId) {
      span.fail('no_connected_pty', { terminal: params.terminal })
      throw new Error('no_connected_pty')
    }
    try {
      const result = await runtime.resizeForClient(
        leaf.ptyId,
        params.mode,
        params.clientId,
        params.mode === 'mobile-fit' ? params.cols : undefined,
        params.mode === 'mobile-fit' ? params.rows : undefined
      )
      span.ok({ ptyId: leaf.ptyId })
      return {
        terminal: {
          handle: params.terminal,
          ...result
        }
      }
    } catch (err) {
      span.fail(err, { terminal: params.terminal })
      throw err
    }
  }
})
```

> `terminal.split` dùng lại `Tracers.terminalCreate` (không tạo tracer `terminal:split` riêng) — đúng nguyên tắc CR-TRACE-003 §3: split về bản chất là một lần create PTY, chỉ khác context UI.

### 2.3 Provider-level instrumentation — `LocalPtyProvider` / `SshPtyProvider`

Vì `OrcaRuntimeService.createTerminal()` (26K dòng, nhiều nhánh `shouldCreateInBackground`/renderer-backed) không có một điểm "chọn provider" đơn giản để chèn `span.step()` an toàn mà không đọc toàn bộ hàm, solution này chọn instrument **trực tiếp bên trong từng provider's `spawn()`** — mỗi provider tự biết `providerType` của chính nó, không cần suy luận ngược từ RPC layer:

```typescript
// src/main/providers/local-pty-provider.ts
import { Tracers } from '../../shared/trace/tracers'

export class LocalPtyProvider implements IPtyProvider {
  // ...existing fields unchanged...

  async spawn(args: PtySpawnOptions): Promise<PtySpawnResult> {
    // Why: span không nhận traceId ở đây — PtySpawnOptions không có field trace
    // (interface dùng chung cho cả 2 provider, thay đổi shape sẽ ảnh hưởng rộng).
    // Thay vào đó dùng span rời (không resume) chỉ để đo latency + phân biệt provider,
    // join với span RPC cha (terminal:create) qua field ptyId ở phía log/TracePanel.
    const span = Tracers.terminalCreate.start({ providerType: 'local', step: 'provider-spawn' })
    try {
      // ...existing reattach / allocatePtyId / spawn logic unchanged...
      const result = /* existing return value */
      span.ok({ providerType: 'local', ptyId: result.id })
      return result
    } catch (err) {
      span.fail(err, { providerType: 'local' })
      throw err
    }
  }
}
```

```typescript
// src/main/providers/ssh-pty-provider.ts
import { Tracers } from '../../shared/trace/tracers'

export class SshPtyProvider implements IPtyProvider {
  // ...existing fields unchanged...

  async spawn(opts: PtySpawnOptions): Promise<PtySpawnResult> {
    // Why: theo CR-TRACE-000 §3.3 dòng cuối — traceId KHÔNG lan vào remote shell.
    // Span này chỉ đo phía Main (mux.request('pty.attach'/'pty.create', ...) round-trip
    // qua SshChannelMultiplexer), không kỳ vọng resume tiếp bên trong remote host.
    const span = Tracers.terminalCreate.start({ providerType: 'ssh', step: 'provider-spawn' })
    try {
      // ...existing reattach-via-pty.attach / spawn-via-pty.create logic unchanged...
      const result = /* existing return value */
      span.ok({ providerType: 'ssh', ptyId: result.id })
      return result
    } catch (err) {
      span.fail(err, { providerType: 'ssh' })
      throw err
    }
  }
}
```

> **Lưu ý thiết kế:** 2 span này **không `resume`** từ span `terminal:create` ở RPC layer (mục 2.2) vì `PtySpawnOptions` không mang `traceId` xuyên qua `createTerminal()` — refactor `PtySpawnOptions` để thêm field đó có blast radius lớn (dùng chung bởi daemon adapter, agent-teams launch, background terminal spawn, ...) và **nằm ngoài phạm vi "additive-only, không đổi business logic"** của CR-TRACE-003. Đây là gap được flag rõ ràng thay vì fabricate một cách xuyên field không có cơ sở: 2 span `provider-spawn` xuất hiện **độc lập** trong TracePanel, join bằng field `ptyId` chung (giống pattern `worktree.checkSafety`/`worktree.rm` ở SOL-BE-TRACE-001 §2.3) thay vì bằng `id` xuyên suốt. Nếu về sau `PtySpawnOptions` được mở rộng thêm `traceId?`, đổi 2 đoạn code trên sang dùng `resume` là thay đổi nhỏ, không ảnh hưởng gì khác.

### 2.4 `src/main/terminal-scrollback-snapshots.ts` — instrument save path

```typescript
import { Tracers } from '../shared/trace'
// Lưu ý: file này nằm dưới src/main/ (không phải rpc/methods), import trực tiếp
// từ '../shared/trace/tracers' theo path tương đối đúng vị trí file.

export function migrateWorkspaceSessionTerminalScrollbackSnapshots(
  session: WorkspaceSessionState,
  storage?: TerminalScrollbackSnapshotStorage
): { session: WorkspaceSessionState; changed: boolean } {
  // Why (CR-TRACE-003): guard trước khi start span — hàm này được gọi mỗi lần
  // Store.setWorkspaceSession()/load() chạy, phần lớn lời gọi không có buffer nào
  // cần ghi (đã migrate từ trước). Chỉ tạo span khi thực sự có sync I/O sắp chạy,
  // tránh vi phạm nguyên tắc chống over-instrumentation (CR-TRACE-000 §5).
  const hasPendingBuffers = Object.values(session.terminalLayoutsByTabId ?? {}).some(
    (layout) => layout.buffersByLeafId && Object.keys(layout.buffersByLeafId).length > 0
  )
  if (!hasPendingBuffers) {
    // ...existing early-return / no-op path unchanged (return { session, changed: false })...
  }

  const span = Tracers.terminalDestroy.start({ step: 'migrate-scrollback-snapshots' })
  let bytesWritten = 0
  let terminalLayoutsByTabId: WorkspaceSessionState['terminalLayoutsByTabId'] | null = null
  try {
    for (const [tabId, layout] of Object.entries(session.terminalLayoutsByTabId ?? {})) {
      const buffers = layout.buffersByLeafId
      if (!buffers || Object.keys(buffers).length === 0) {
        continue
      }
      const refs = { ...layout.scrollbackRefsByLeafId }
      const remainingBuffers: Record<string, string> = {}
      let layoutChanged = false
      for (const [leafId, buffer] of Object.entries(buffers)) {
        span.step('write-snapshot-sync', { tabId, leafId, bufferBytes: buffer.length })
        const ref = writeTerminalScrollbackSnapshotSync({ tabId, leafId, buffer, storage })
        if (ref) {
          bytesWritten += buffer.length
          refs[leafId] = ref
        }
        // ...existing refs/remainingBuffers/layoutChanged bookkeeping unchanged...
      }
      // ...existing terminalLayoutsByTabId assembly unchanged...
    }
    span.ok({ bytesWritten })
    // ...existing return { session: {...}, changed } unchanged...
  } catch (err) {
    span.fail(err, { bytesWritten })
    throw err
  }
}
```

> **Điểm quan trọng khác với minh hoạ của CR-TRACE-003 §4:** CR giả định 1 span bao cả "write-snapshot-sync" + "provider-destroy" trên **1 PTY**. Thực tế `migrateWorkspaceSessionTerminalScrollbackSnapshots()` xử lý **N PTY cùng lúc** (toàn bộ session), và **không** gọi `provider.destroy()` ở đây — destroy PTY là một luồng runtime riêng (`runtime.closeTerminal()`/`stopTerminalsForWorktree()`, chưa được CR-TRACE-003 hay solution này instrument vì không nằm trong danh sách file CR liệt kê). SOL này chỉ trace phần ghi scrollback thật sự tồn tại trong code, không fabricate bước "provider-destroy" gộp chung.

### 2.5 `src/main/ipc/session.ts` — instrument restore path (Electron IPC, in-process)

```typescript
import { Tracers } from '../../shared/trace/tracers'

ipcMain.on(
  'session:read-terminal-scrollback-sync',
  (event, args: { ref?: unknown } | undefined) => {
    const ref = typeof args?.ref === 'string' ? args.ref : null
    if (!ref) {
      event.returnValue = null
      return
    }
    const span = Tracers.terminalReconnect.start({ ref })
    try {
      span.step('read-snapshot-sync', { ref })
      const buffer = store.readTerminalScrollbackSnapshot(ref)
      span.ok({ ref, restoredBytes: buffer?.length ?? 0 })
      event.returnValue = buffer
    } catch (err) {
      span.fail(err, { ref })
      event.returnValue = null
    }
  }
)
```

> Đây là lời gọi **đồng bộ, cùng process** (Electron main, `event.returnValue` — synchronous IPC) — không có `traceId` wire nào để propagate (chỉ 1 process tham gia, theo CR-TRACE-000 §3.3 các dòng "in-process"). Không có transport row nào trong CR-TRACE-000 §3.3 khớp chính xác "Electron ipcMain đồng bộ" — solution này xử lý như in-process, tương tự cách CR-TRACE-001 xử lý `ProjectServerRouter`/`RelayConnectionPool` (§4 bảng "Thành phần & Transport").

---

## 3. Test Plan (Vitest)

| Test file | Test case | Mục tiêu |
|---|---|---|
| `src/shared/trace/__tests__/tracers.test.ts` | `'exports Tracers.terminalCreate/Resize/Destroy/Reconnect with correct flow names'` | Convention CR-TRACE-000 §4 |
| `src/main/runtime/rpc/methods/__tests__/terminal.test.ts` | `'terminal.create emits terminalCreate span with ok() containing ptyId'` | |
| (cùng file) | `'terminal.create resumes span id from params.traceId'` | |
| (cùng file) | `'terminal.split reuses Tracers.terminalCreate, not a separate tracer'` | Guard chống tạo `terminal:split` riêng |
| (cùng file) | `'terminal.resizeForClient emits terminalResize span distinct from terminalCreate'` | |
| (cùng file) | `'terminal.resizeForClient span.fail() called on no_connected_pty before throwing'` | |
| `src/main/providers/__tests__/local-pty-provider.test.ts` | `'spawn() emits terminalCreate span with providerType=local'` | |
| (cùng file) | `'spawn() span.fail() on underlying pty spawn error, providerType field present'` | |
| `src/main/providers/__tests__/ssh-pty-provider.test.ts` | `'spawn() emits terminalCreate span with providerType=ssh'` | |
| (cùng file) | `'spawn() span does not attempt to propagate traceId into remote shell (no wire field added)'` | Guard xác nhận đúng CR-TRACE-000 §3.3 dòng SSH exec |
| `src/main/__tests__/terminal-scrollback-snapshots.test.ts` | `'migrateWorkspaceSessionTerminalScrollbackSnapshots skips span entirely when no buffersByLeafId pending'` | Guard chống over-instrumentation |
| (cùng file) | `'migrateWorkspaceSessionTerminalScrollbackSnapshots emits terminalDestroy span with step write-snapshot-sync per leaf, ok() bytesWritten aggregated'` | |
| (cùng file) | `'migrateWorkspaceSessionTerminalScrollbackSnapshots span.fail() on writeTerminalScrollbackSnapshotSync throw'` | |
| `src/main/ipc/__tests__/session.test.ts` | `'session:read-terminal-scrollback-sync emits terminalReconnect span with restoredBytes'` | |
| (cùng file) | `'session:read-terminal-scrollback-sync span.ok() with restoredBytes=0 when ref not found (buffer null)'` | |
| (cùng file) | `'session:read-terminal-scrollback-sync does not start a span when ref is missing/invalid'` | |
| `src/shared/__tests__/terminal-osc133-command-finished.test.ts` (regression, không phải test mới) | `'no Tracers.* call anywhere in osc133 scanning path'` | Xác nhận BL-TM-04 vẫn không bị instrument (grep-based guard test) |

**Test Targets:**

| Nhóm | Target |
|---|---|
| `tracers.test.ts` (mở rộng) | ≥ 2 |
| `terminal.test.ts` (mở rộng) | ≥ 5 |
| `local-pty-provider.test.ts` (mở rộng) | ≥ 2 |
| `ssh-pty-provider.test.ts` (mở rộng) | ≥ 2 |
| `terminal-scrollback-snapshots.test.ts` (mở rộng) | ≥ 3 |
| `session.test.ts` (mới hoặc mở rộng) | ≥ 3 |
| Guard test OSC133 | 1 |
| **Total** | **≥ 18** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.terminalCreate`, `terminalResize`, `terminalDestroy`, `terminalReconnect` tồn tại trong `tracers.ts` đúng flow name `terminal:create|resize|destroy|reconnect`
- [ ] Handler `terminal.create` và `terminal.split` (`terminal.ts`) đều dùng `Tracers.terminalCreate` — không có tracer `terminal:split` riêng
- [ ] `LocalPtyProvider.spawn()` và `SshPtyProvider.spawn()` đều phát span `terminal:create` với field `providerType` phân biệt (`'local'`/`'ssh'`), cho phép biết ngay trong TracePanel/log một terminal chậm là do provider nào mà không cần hỏi lại user
- [ ] `migrateWorkspaceSessionTerminalScrollbackSnapshots()` chỉ tạo span khi thực sự có `buffersByLeafId` cần ghi (guard chống over-instrumentation, verify bằng test — hàm này được gọi trên mọi `setWorkspaceSession()`)
- [ ] `span.step('write-snapshot-sync', { bufferBytes })` bao quanh mỗi lời gọi `writeTerminalScrollbackSnapshotSync()` — đúng tiêu chí CR-TRACE-000 §5 (sync I/O có khả năng block event loop)
- [ ] `session:read-terminal-scrollback-sync` (Electron IPC, `ipc/session.ts`) phát span `terminal:reconnect` với `restoredBytes`
- [ ] KHÔNG có span/tracer nào được tạo cho việc scan OSC 133 trên mỗi PTY chunk (BL-TM-04) — verify bằng test guard dựa trên grep
- [ ] Solution document ghi rõ (mục 1.1/1.3) rằng kiến trúc thật có 3 con đường PTY và scrollback đi qua Electron IPC chứ không phải `relay.call('pty.*')`/DB như flow doc gốc mô tả — khuyến nghị đồng thời cập nhật `docs/flows/logic/terminal-management.md`
