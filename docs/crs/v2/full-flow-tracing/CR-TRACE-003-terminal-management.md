# CR-TRACE-003 — Terminal Management Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-003 |
| **Tên** | Terminal Management — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/terminal-management.md`, `src/main/runtime/rpc/methods/terminal.ts`, `src/main/runtime/orca-runtime.ts`, `src/main/providers/local-pty-provider.ts`, `src/main/providers/ssh-pty-provider.ts`, `src/main/terminal-scrollback-snapshots.ts`, `src/relay/agent-rpc-dispatch.ts`, `src/shared/terminal-osc133-command-finished.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

`docs/flows/logic/terminal-management.md` mô tả một kiến trúc "3-tier" nơi PTY luôn chạy qua `relay.call('pty.create'|'pty.resize'|'pty.destroy'|'pty.scrollback')` tới Dev Server, điều phối bởi một `WorkspaceContextManager`. Khi đối chiếu với source thật:

1. **`WorkspaceContextManager` không tồn tại.** RPC method `terminal.create` (`src/main/runtime/rpc/methods/terminal.ts:1285-1315`) gọi thẳng `runtime.createTerminal()` trong `OrcaRuntimeService` (`src/main/runtime/orca-runtime.ts:17470`) — không có lớp điều phối riêng tên như flow doc mô tả.
2. **Không có lời gọi `relay.call('pty.create', ...)` nào ở phía Orca Server** cho terminal thông thường (đã grep toàn bộ `src/` cho `relay.call('pty.` và `.call('pty.` — không thấy caller nào ngoài case handler phía Dev Server trong `agent-rpc-dispatch.ts:630-706`). PTY thật chạy qua 2 provider khác hẳn kiến trúc "Dev Server relay" trong doc: `LocalPtyProvider` (`src/main/providers/local-pty-provider.ts:312`, spawn cục bộ trên máy chạy Electron main) và `SshPtyProvider` (`src/main/providers/ssh-pty-provider.ts:35`, chạy qua kênh SSH thuần tới remote host). Cả hai đều **không dùng khái niệm "Dev Server Agent" (WS-based) mà terminal-management.md mô tả** — khái niệm đó (agent-rpc-dispatch.ts's `pty.create` case) hiện chỉ thấy được gọi trong ngữ cảnh agent-orchestration (`agent.spawn`/`agent.exec` tạo PTY cho AI agent, xem CR-TRACE-002), không phải cho terminal người dùng mở tay.
3. Đây là một **khác biệt kiến trúc thực sự giữa tài liệu và code**, không chỉ thiếu tracing: có 3 con đường PTY khác nhau (`LocalPtyProvider`, `SshPtyProvider`, và `agent-rpc-dispatch.ts`'s `pty.*` dùng riêng cho AI agent) nhưng flow doc mô tả như thể chỉ có 1. Không có tracing ở bất kỳ đường nào trong 3, nên khi người dùng báo "mở terminal bị chậm/treo", **không cách nào từ log biết được request đi qua provider nào** (`Local`, `Ssh`, hay agent-relay `pty.create`) trước khi phải hỏi lại user để tái hiện.
4. **Scrollback (BL-TM-03)**: mã thật (`src/main/terminal-scrollback-snapshots.ts`) lưu scrollback vào **file cục bộ, đồng bộ** (`writeTerminalScrollbackSnapshotSync`, `readTerminalScrollbackSnapshotSync`) — khác hẳn mô tả flow doc ("`relay.call('pty.scrollback')` → gzip → INSERT DB `orca_terminal_scrollback_snapshots`"). Việc ghi file đồng bộ trên I/O có thể block event loop nếu buffer lớn; không có timing nào để phát hiện việc này hôm nay.
5. **OSC 133 parsing (BL-TM-04)**: có scanner thật `src/shared/terminal-osc133-command-finished.ts` (chunk-boundary-safe OSC 133;D scanner), nhưng đây là logic **thuần in-memory, chạy cả ở main và renderer tuỳ loại PTY** — không phải một `OSC133Parser` tập trung ở "Orca Server" như flow doc vẽ. Không cần trace các lệnh scan text bình thường (theo CR-TRACE-000 §5 — biến đổi in-memory thuần tuý), nhưng **việc dispatch structured event `shell:commandFinished`/`agent:toolCallStarted` ra ngoài (IPC/WS push) thì đáng trace** vì đó là điểm cross-boundary.

Tóm lại: vấn đề cụ thể cần giải quyết bằng CR này là mọi round-trip PTY qua 3 loại provider hiện tại đều mù hoàn toàn về thời gian/lỗi, và tên tracer cần phản ánh đúng provider thực tế (không giả định một "Dev Server relay" chung như tài liệu cũ).

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------|-------|-----------|-------------------------------|
| Browser (xterm.js) | UI | WebSocket RPC (Browser ↔ Orca Server) | "WebSocket RPC (Browser ↔ Orca Server)" |
| `src/main/runtime/rpc/methods/terminal.ts` (`terminal.create`, `.split`, `.resizeForClient`, `.close`) | Business Logic | WebSocket RPC | "WebSocket RPC (Browser ↔ Orca Server)" |
| `OrcaRuntimeService.createTerminal()` (`src/main/runtime/orca-runtime.ts:17470`) | Business Logic | in-process, chọn provider | — |
| `LocalPtyProvider` (`src/main/providers/local-pty-provider.ts:312`) | Runtime (local) | in-process node-pty spawn, không băng qua network | — (không cần propagation, nhưng vẫn đáng `step()` vì có thể chậm/fail) |
| `SshPtyProvider` (`src/main/providers/ssh-pty-provider.ts:35`) | Runtime (remote) | SSH exec / channel | "SSH exec / `SshChannelMultiplexer`" — traceId KHÔNG lan vào remote shell, chỉ trace phía Main (connect/channel-open) |
| `src/relay/agent-rpc-dispatch.ts` case `pty.create/resize/destroy/scrollback` (dùng cho AI agent PTY, xem CR-TRACE-002) | Runtime (Dev Server) | Agent WS JSON-RPC 2.0 | "Agent WS JSON-RPC 2.0" |
| `src/main/terminal-scrollback-snapshots.ts` | Persistence (file cục bộ) | in-process sync I/O | — |
| `src/shared/terminal-osc133-command-finished.ts` | Business Logic (parse) | in-process | — |
| Server DB (`orca_terminal_sessions`) | Persistence | in-process | — |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  terminalCreate:    createTracer('terminal:create'),    // BL-TM-01
  terminalResize:    createTracer('terminal:resize'),    // BL-TM-02 (split dùng lại terminalCreate; resize riêng)
  terminalDestroy:   createTracer('terminal:destroy'),   // BL-TM-03 (save+destroy scrollback)
  terminalReconnect: createTracer('terminal:reconnect'), // BL-TM-03 (restore flow)
}
```

> Ghi chú đặt tên: BL-TM-02 (split) trong flow doc thực chất tái sử dụng cùng cơ chế `terminal.create` (một PTY mới) — instrumentation dùng lại `terminalCreate` (không tạo tracer `terminal:split` riêng, theo quy ước "1 tracer = 1 sub-flow" nhưng split về bản chất **là** một lần create, chỉ khác context UI). `terminal:resize` được giữ riêng cho các API resize tần suất thấp/điểm rẽ nhánh (không phải mọi keystroke). BL-TM-04 (OSC 133 + agent hook) **không có tracer riêng** — xem lý do ở mục 4.

## 4. Instrumentation theo từng sub-flow

### BL-TM-01 — Tạo PTY Session (Remote/Local)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `terminal.create` | `start` | `{ worktree, cols, rows }` | `src/main/runtime/rpc/methods/terminal.ts:1285` (handler) |
| Chọn provider (Local/SSH) | `step('select-provider')` | `{ providerType: 'local' \| 'ssh' }` | `src/main/runtime/orca-runtime.ts:17470` (`createTerminal()`) |
| Spawn PTY qua provider | `step('provider-spawn')` | `{ providerType }` | `LocalPtyProvider` (`local-pty-provider.ts:312`) hoặc `SshPtyProvider` (`ssh-pty-provider.ts:35`) — method spawn cụ thể chưa xác định tên hàm chính xác, cần xác nhận thêm khi implement |
| INSERT session | không `step()` (single-row INSERT) — gộp vào `ok()` | `{ ptyId }` | chưa xác định file cụ thể cho INSERT `orca_terminal_sessions` |
| Hoàn tất | `ok`/`fail` | `{ ptyId, providerType }` | — |

```typescript
// src/main/runtime/rpc/methods/terminal.ts — handler 'terminal.create'
handler: async (params, ctx) => {
  if (!ctx.userId) throw new Error('UNAUTHORIZED: terminal.create requires an authenticated session. Please log in.')
  const span = Tracers.terminalCreate.start(
    { worktree: params.worktree, cols: params.cols, rows: params.rows },
    params.traceId ? { id: params.traceId } : undefined
  )
  try {
    const terminal = await ctx.runtime.createTerminal(params.worktree, { /* ...existing options... */ })
    span.ok({ ptyId: terminal.ptyId })
    return { terminal }
  } catch (err) {
    span.fail(err, { worktree: params.worktree })
    throw err
  }
}
```

> **Vì SSH exec không lan truyền `traceId` vào remote shell (CR-TRACE-000 §3.3 dòng cuối)**, `span.step('provider-spawn')` cho nhánh `SshPtyProvider` chỉ đo latency phía Main (connect + `exec` channel open), không kỳ vọng thấy trace tiếp tục bên trong shell của remote host.

### BL-TM-02 — Split Terminal

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `terminal.split` | `start` (tái dùng `Tracers.terminalCreate`, không phải tracer riêng) | `{ terminal, direction }` | `src/main/runtime/rpc/methods/terminal.ts:1317` (handler `terminal.split`) → `runtime.splitTerminal()` |
| Spawn PTY mới cho pane | `step('provider-spawn')` | `{ direction }` | tương tự BL-TM-01 |
| Hoàn tất | `ok` | `{ ptyId }` | — |

```typescript
// src/main/runtime/rpc/methods/terminal.ts — handler 'terminal.split'
handler: async (params, { runtime }) => {
  const span = Tracers.terminalCreate.start({ terminal: params.terminal, direction: params.direction })
  try {
    const split = await runtime.splitTerminal(params.terminal, { direction: params.direction, command: params.command, env: params.env, telemetrySource: params.telemetrySource })
    span.ok({ ptyId: split.ptyId })
    return { split }
  } catch (err) {
    span.fail(err, { terminal: params.terminal })
    throw err
  }
}
```

Resize (`terminal.resizeForClient`, `terminal.ts:1343`) dùng tracer riêng vì tần suất/độ nhạy khác (mỗi lần user kéo pane, không phải mỗi phím gõ):

```typescript
handler: async (params, { runtime }) => {
  const span = Tracers.terminalResize.start({ terminal: params.terminal, cols: params.cols, rows: params.rows })
  try {
    const result = await runtime.resizeForClient(params.terminal, params.cols, params.rows /* ... */)
    span.ok()
    return result
  } catch (err) {
    span.fail(err, { terminal: params.terminal })
    throw err
  }
}
```

### BL-TM-03 — Lưu và Khôi phục Scrollback

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| **SAVE**: bắt đầu save | `start` (`terminal:destroy`) | `{ ptyId }` | chưa xác định file cụ thể gọi `writeTerminalScrollbackSnapshotSync` (hàm tồn tại ở `src/main/terminal-scrollback-snapshots.ts:98` nhưng call site khi workspace deactivate chưa xác nhận) |
| Ghi file scrollback (sync I/O) | `step('write-snapshot-sync')` | `{ ptyId, bufferBytes }` | `src/main/terminal-scrollback-snapshots.ts:98` (`writeTerminalScrollbackSnapshotSync`) — **đáng trace vì đây là sync I/O có thể block event loop, đúng tiêu chí CR-TRACE-000 §5 mục 2 "có khả năng chậm"** |
| Destroy PTY | `step('provider-destroy')` | `{ ptyId }` | provider tương ứng (`LocalPtyProvider`/`SshPtyProvider`) |
| Hoàn tất SAVE | `ok`/`fail` | `{ ptyId }` | — |
| **RESTORE**: bắt đầu restore | `start` (`terminal:reconnect`) | `{ tabId, leafId }` | chưa xác định file cụ thể |
| Đọc file scrollback | `step('read-snapshot-sync')` | `{ ref }` | `src/main/terminal-scrollback-snapshots.ts:136` (`readTerminalScrollbackSnapshotSync`) |
| Spawn PTY mới + inject buffer | `step('provider-spawn-with-restore')` | `{ providerType }` | tương tự BL-TM-01 |
| Hoàn tất RESTORE | `ok`/`fail` | `{ ptyId, restoredBytes }` | — |

```typescript
// Ví dụ tại call site save (file chưa xác định — minh hoạ theo hàm thật writeTerminalScrollbackSnapshotSync)
const span = Tracers.terminalDestroy.start({ ptyId })
try {
  span.step('write-snapshot-sync', { ptyId, bufferBytes: buffer.length })
  writeTerminalScrollbackSnapshotSync({ ptyId, buffer /* ...existing args... */ })
  span.step('provider-destroy', { ptyId })
  await provider.destroy(ptyId)
  span.ok({ ptyId })
} catch (err) {
  span.fail(err, { ptyId })
  throw err
}
```

> Lưu ý: `writeTerminalScrollbackSnapshotSync`/`readTerminalScrollbackSnapshotSync` là hàm **đồng bộ** (`Sync` trong tên) — không có `relay.call` nào ở đây như flow doc mô tả (không có gzip qua network, không có bảng `orca_terminal_scrollback_snapshots` được xác nhận tồn tại qua grep). Nếu kiến trúc thật khác biệt đủ lớn so với doc, khuyến nghị cập nhật `terminal-management.md` song song với việc triển khai CR này để hai tài liệu không tiếp tục lệch nhau.

### BL-TM-04 — Shell Integration (OSC 133) + Agent Hook

Theo CR-TRACE-000 §5, việc scan text OSC 133 (`src/shared/terminal-osc133-command-finished.ts`) là **biến đổi in-memory thuần tuý trên từng chunk PTY output** — tần suất cực cao (mỗi vài chục ms khi user gõ lệnh), **không đáng một `span.step()` riêng cho mỗi lần scan**. Không tạo tracer domain riêng cho BL-TM-04.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Scan OSC 133 trong PTY chunk | KHÔNG trace (in-memory, tần suất cao) | — | `src/shared/terminal-osc133-command-finished.ts` |
| Dispatch structured event `shell:commandFinished`/`agent:toolCallFinished` ra IPC/WS (cross-boundary, có thể đáng chú ý nếu nhiều listener) | Cân nhắc gộp vào field của span cha nếu đang có PTY session span mở (ví dụ đo qua counter thay vì tracer riêng) | `{ ptyId, exitCode }` | chưa xác định file cụ thể phát các event này ở phía Orca Server |

> **Khuyến nghị:** nếu cần debug hiệu năng OSC-hook cụ thể (ví dụ dispatch bị dồn ứ khi output dày), nên dùng một counter/metric riêng (ngoài phạm vi `span.step()`) thay vì mở tracer mới, để không vi phạm nguyên tắc chống over-instrumentation của CR-TRACE-000 §5.

## 5. Lan truyền traceId qua transport của flow này

1. **Browser → Orca Server (`terminal.create`/`terminal.split`/`terminal.resizeForClient`)**: giống CR-TRACE-001/002 — thêm `traceId?: string` optional vào params schema tương ứng trong `terminal.ts`, không đụng `RpcRequest.id` (`src/main/runtime/rpc/core.ts:33-38`) vốn dùng cho request/response matching.
2. **`OrcaRuntimeService.createTerminal()` → `LocalPtyProvider`/`SshPtyProvider`**: đây là lời gọi **in-process**, không băng qua network — không cần field wire, chỉ cần truyền `span` (hoặc `span.id`) xuống qua tham số hàm nếu provider cần log lại chính id đó khi emit lỗi async (PTY exit event sau này).
3. **`SshPtyProvider` → remote host qua SSH exec**: theo CR-TRACE-000 §3.3 dòng cuối, **không lan truyền `traceId` vào remote shell** — chỉ trace phía Main (`ssh2.connect`, mở channel). Đây là ranh giới cứng, không có cách nào vượt qua vì remote shell không chạy code Orca.
4. **Agent-PTY qua `agent-rpc-dispatch.ts` (`pty.create` case, dùng khi AI agent cần PTY riêng — xem CR-TRACE-002 BL-AG-01)**: nếu terminal UI sau này hiển thị PTY của agent, dùng đúng convention Agent WS JSON-RPC 2.0 (`params._trace.id`) như CR-TRACE-002 §5 mục 4, KHÔNG dùng field `traceId` phẳng.
5. **Scrollback save/restore**: hoàn toàn in-process (file I/O cục bộ), không có transport nào để propagate — span chỉ tồn tại trong 1 process, không cần `resume`.

## Acceptance Criteria

- [ ] `Tracers.terminalCreate`, `terminalResize`, `terminalDestroy`, `terminalReconnect` được thêm vào `tracers.ts` đúng tên `terminal:create|resize|destroy|reconnect`
- [ ] Handler `terminal.create` và `terminal.split` (`terminal.ts`) đều dùng `Tracers.terminalCreate`, phân biệt qua field `direction` (có/không) trong `ok()`, không tạo tracer `terminal:split` riêng
- [ ] `span.step('provider-spawn', { providerType })` cho phép phân biệt ngay trong TracePanel/log một terminal chậm là do `LocalPtyProvider` hay `SshPtyProvider`, không cần hỏi lại user để tái hiện
- [ ] `writeTerminalScrollbackSnapshotSync`/`readTerminalScrollbackSnapshotSync` có `span.step()` bao quanh với field `bufferBytes`, vì đây là sync I/O có khả năng block event loop (đúng tiêu chí CR-TRACE-000 §5)
- [ ] KHÔNG có span/tracer nào được tạo cho việc scan OSC 133 trên mỗi PTY chunk (BL-TM-04) — xác nhận bằng code review, tránh vi phạm nguyên tắc chống over-instrumentation
- [ ] `SshPtyProvider` step ghi rõ trong field/comment rằng `traceId` dừng lại ở Main process, không lan vào remote shell
- [ ] Tài liệu `docs/flows/logic/terminal-management.md` được gắn cờ review lại kiến trúc (mismatch `WorkspaceContextManager`/relay `pty.*` vs. `LocalPtyProvider`/`SshPtyProvider` thật) song song hoặc trước khi triển khai instrumentation, để tracer name phản ánh đúng thực tế thay vì tài liệu cũ
