# SOL-AG-TRACE-003: Terminal Management — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**TDD Ref:** TDD-AG-01 (Architecture & Process Model), TDD-AG-12 (ProfileAware Agent Spawner)
**File(s):**
- `src/shared/trace/tracers.ts` [MODIFY] (idempotent với SOL-AG-TRACE-001, nếu đã tồn tại thì bỏ qua)
- `src/relay/pty-agent-bridge.ts` [MODIFY]
**Mức độ:** 🟢 Đơn giản
**Thời gian ước tính:** 2.5h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only) — CHÚ Ý: phạm vi hẹp hơn các CR khác

CR-TRACE-003 §1 mục 2-3 đã tự xác nhận: **PTY người dùng thông thường (terminal thường mở tay) KHÔNG chạy qua `src/relay/agent-*.ts`**. Có 3 con đường PTY hoàn toàn tách biệt trong hệ thống:

| Con đường | File | Dùng cho | Chạy trong `src/relay/`? |
|-----------|------|----------|----------------------------|
| `LocalPtyProvider` | `src/main/providers/local-pty-provider.ts` | Terminal người dùng, chế độ desktop/local | ❌ Không — chạy trong Electron Main |
| `SshPtyProvider` | `src/main/providers/ssh-pty-provider.ts` | Terminal người dùng, qua SSH thuần | ❌ Không — chạy trong Electron Main, dùng kênh SSH exec |
| `pty.create/write/resize/destroy/scrollback/sendSignal` (agent-rpc-dispatch.ts case, L626-703) | `src/relay/pty-agent-bridge.ts` | **CHỈ dùng cho PTY riêng của AI agent** (comment thật trong file: "TM-001/TM-006: PTY management for **agent RPC mode**") | ✅ Có — đây là toàn bộ phạm vi của solution này |

Do đó **solution này KHÔNG đụng đến terminal người dùng thông thường** — điều đó nằm ngoài `src/relay/`, thuộc một solution set khác (Electron Main / Orca Server), nếu tồn tại. Toàn bộ code thay đổi ở đây chỉ nằm trong **1 file duy nhất**: `src/relay/pty-agent-bridge.ts`.

**Quan hệ với CR-TRACE-002:** CR-TRACE-003 §5 mục 4 tự nhận định con đường `pty.create` qua `agent-rpc-dispatch.ts` "dùng khi AI agent cần PTY riêng — xem CR-TRACE-002 BL-AG-01". Về bản chất kiến trúc, `pty-agent-bridge.ts` phục vụ Agent Orchestration (hiển thị PTY của AI agent), không phải Terminal Management theo đúng nghĩa BL-TM. Solution này vẫn dùng tên tracer `terminal:*` (đã được CR-TRACE-003 §3 đặt riêng cho việc tạo/resize/destroy PTY, bất kể provider nào) để giữ đúng namespace đã quy ước, nhưng gắn thêm field `origin: 'agent-pty'` để phân biệt trong TracePanel với các span `terminal:create` khác (nếu sau này backend-side solution cũng dùng chung tên cho `LocalPtyProvider`/`SshPtyProvider`) — 2 nguồn này không bao giờ cùng phát sinh cho 1 request, nhưng field giúp đọc log thô rõ ràng hơn.

**Ngoài phạm vi (backend/gateway hoặc Electron Main, solution set khác):**
- `src/main/runtime/rpc/methods/terminal.ts` (`terminal.create/.split/.resizeForClient/.close`)
- `src/main/runtime/orca-runtime.ts` (`createTerminal()`)
- `src/main/providers/local-pty-provider.ts`, `src/main/providers/ssh-pty-provider.ts`
- `src/main/terminal-scrollback-snapshots.ts` (cơ chế scrollback **khác** — file snapshot đồng bộ cho terminal người dùng, không liên quan đến buffer scrollback trong-memory của `pty-agent-bridge.ts`, xem mục 2)
- `src/shared/terminal-osc133-command-finished.ts` — không dùng trong `src/relay/agent-*.ts` (chỉ dùng ở `src/relay/pty-shell-launch.ts`, thuộc relay daemon thường, không phải Agent RPC path)

**Điều kiện tiên quyết:** giống 2 solution trước — giả định `Tracer.start(fields?, resume?)` (CR-TRACE-000 §3.1) đã ship.

### Quyết định thiết kế: không trace `pty.write` và `pty.scrollback`

- **`pty.write`** (mỗi keystroke gửi vào PTY tương tác) — tần suất cực cao, đúng loại "biến đổi/forward tần suất cao" mà CR-TRACE-000 §5 và CR-TRACE-002 (BL-AG-05, nhánh `agent.sendInput` không phải Ctrl+C) đã loại trừ. KHÔNG thêm span.
- **`pty.scrollback`** — đọc buffer **trong bộ nhớ** (`entry.buf`, một mảng string được `Array.slice()`), không có I/O, không băng qua process boundary nào khác ngoài chính request/response RPC (đã được `agent:rpc` tracer hạ tầng bọc sẵn). Đây là "biến đổi dữ liệu in-memory thuần tuý" theo đúng định nghĩa loại trừ ở CR-TRACE-000 §5 mục cuối. **Khác** với cơ chế scrollback CR-TRACE-003 mô tả cho BL-TM-03 (`writeTerminalScrollbackSnapshotSync`/`readTerminalScrollbackSnapshotSync` — sync file I/O, ĐÁNG trace) — đó là cơ chế khác, ở file khác, ngoài phạm vi solution này.

## 2. Gap hiện tại

| Vị trí | Hiện trạng | Gap |
|--------|-----------|-----|
| `agent-rpc-dispatch.ts` case `'pty.create'` (L630-638) / `'pty.resize'` (L656-664) / `'pty.destroy'` (L669-677) / `'pty.sendSignal'` (L695-703) | Chỉ bọc bởi tracer hạ tầng chung `agent:rpc` — `extractTraceFields()` (`agent-rpc-dispatch.ts:58-118`) **không có nhánh nào cho `pty.*`**, rơi vào `return {}` mặc định (L117) | Không field nào (`ptyId`, `cols`, `rows`) xuất hiện trong span `agent:rpc` cho các method này; không domain span `terminal:*` |
| `pty-agent-bridge.ts` — `handlePtyCreate()` (L72-123), `handlePtyResize()` (L154-176), `handlePtyDestroy()` (L182-204), `handlePtySendSignal()` (L231-253) | **Không có tracer nào trong toàn bộ file** (xác nhận qua grep `createTracer` — 0 kết quả) | Thiếu hoàn toàn observability cho vòng đời PTY của AI agent — khi PTY agent tạo chậm (do `node-pty` native module load, hoặc `safeCwd()` phải `statSync` một path lạ), không cách nào biết bước nào chậm |
| `src/shared/trace/tracers.ts` | Chưa có (hoặc đã có nếu SOL-AG-TRACE-001 merge trước) | Thiếu `terminalCreate`/`terminalResize`/`terminalDestroy` |
| Toàn bộ file | Không có | Không có `params._trace` extraction |

## 3. Full Implementation

### 3.1 `src/shared/trace/tracers.ts` — thêm entry mới (idempotent)

Nếu SOL-AG-TRACE-001 hoặc backend-side terminal-management solution đã thêm các entry `terminal:*`, bỏ qua block này (cùng file, cùng tên, không xung đột).

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries...

  // ─── CR-TRACE-003: Terminal Management ──────────────────────────────────────
  /** BL-TM-01/02 — create PTY (regular terminal.create/.split, AND agent-PTY pty.create) */
  terminalCreate:  createTracer('terminal:create'),
  /** BL-TM-02 — resize PTY */
  terminalResize:  createTracer('terminal:resize'),
  /** BL-TM-03 — destroy/save-scrollback PTY */
  terminalDestroy: createTracer('terminal:destroy'),
} as const
```

### 3.2 `src/relay/pty-agent-bridge.ts` — resume helper + domain spans

```typescript
// src/relay/pty-agent-bridge.ts
// (thêm vào phần import hiện có)
import type { AgentLogger } from './agent-logger'
import { Tracers } from '../shared/trace/tracers'   // [NEW]

// ─── Trace propagation helper ───────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested tại params._trace.id (CR-TRACE-000 §3.3).
// Hiện tại KHÔNG có caller thật nào gửi _trace cho pty.* (CR-TRACE-003 đã grep xác
// nhận không có relay.call('pty.create', ...) nào ở Orca Server) — helper này là
// "forward-compatible": hoạt động đúng ngay khi caller phía Orca Server được nối vào.
function resumeFrom(params: Record<string, unknown>): { id: string } | undefined {   // [NEW]
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}
```

`handlePtyCreate()` — `Tracers.terminalCreate`:

```typescript
export async function handlePtyCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const cols       = typeof params.cols === 'number' ? params.cols : 80
  const rows       = typeof params.rows === 'number' ? params.rows : 24
  const rawCwd     = typeof params.cwd  === 'string' ? params.cwd  : ''
  const span = Tracers.terminalCreate.start(                                        // [NEW]
    { origin: 'agent-pty', cols, rows },
    resumeFrom(params)
  )

  const nodePty = await import('node-pty').catch(() => null)
  if (!nodePty) {
    span.fail('node-pty not available on this host')                               // [NEW]
    return {
      jsonrpc: '2.0', id,
      error: { code: -32603, message: 'node-pty is not available on this host' },
    }
  }

  const cwd        = safeCwd(rawCwd)
  const envOverride = (params.env && typeof params.env === 'object' && !Array.isArray(params.env))
    ? params.env as Record<string, string>
    : {}

  const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
  const envShell      = typeof envOverride.SHELL     === 'string' ? envOverride.SHELL.trim()   : ''
  const shell = shellOverride || envShell || (process.env.SHELL ?? '/bin/sh')

  const ptyId = `agent-pty-${nextAgentPtyId++}`

  try {
    span.step('node-pty-spawn', { shell, cwd })                                     // [NEW]
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const term = (nodePty as any).spawn(shell, [], {
      name: 'xterm-256color',
      cols,
      rows,
      cwd,
      env: { ...process.env, TERM: 'xterm-256color', ...envOverride } as NodeJS.ProcessEnv,
    })

    const entry: AgentPtyEntry = { pty: term, cwd, cols, rows, shell, buf: '' }
    AGENT_PTY_MAP.set(ptyId, entry)

    term.onData((data: string) => { appendScrollback(entry, data) })

    log.info(`pty.create (agent): id=${ptyId} cwd=${cwd} shell=${shell}`)
    span.ok({ ptyId, shell, cwd })                                                  // [NEW]
    return { jsonrpc: '2.0', id, result: { id: ptyId, cols, rows, cwd, shell } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`pty.create (agent): failed: ${msg}`)
    span.fail(err, { cwd, shell })                                                  // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.create failed: ${msg}` } }
  }
}
```

`handlePtyResize()` — `Tracers.terminalResize`:

```typescript
export async function handlePtyResize(
  id:     string | number | null,
  params: Record<string, unknown>,
  _log:   AgentLogger,
): Promise<object> {
  const ptyId = typeof params.id   === 'string' ? params.id   : ''
  const cols  = typeof params.cols === 'number' ? params.cols : 80
  const rows  = typeof params.rows === 'number' ? params.rows : 24
  const span = Tracers.terminalResize.start({ origin: 'agent-pty', ptyId, cols, rows }, resumeFrom(params))  // [NEW]

  if (!ptyId) {
    span.fail('missing id')                                                         // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.resize: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.fail('pty not found', { ptyId })                                           // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.resize(cols, rows)
    entry.cols = cols
    entry.rows = rows
    span.ok({ ptyId, cols, rows })                                                  // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { ptyId })                                                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.resize failed: ${msg}` } }
  }
}
```

`handlePtyDestroy()` — `Tracers.terminalDestroy`:

```typescript
export async function handlePtyDestroy(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const ptyId   = typeof params.id === 'string' ? params.id : ''
  const graceful = params.graceful !== false
  const span = Tracers.terminalDestroy.start({ origin: 'agent-pty', ptyId, graceful }, resumeFrom(params))  // [NEW]

  if (!ptyId) {
    span.fail('missing id')                                                         // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.destroy: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, alreadyDead: true })                                           // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true, alreadyDead: true } }
  }

  try {
    if (process.platform === 'win32') { entry.pty.kill() }
    else { entry.pty.kill(graceful ? 'SIGTERM' : 'SIGKILL') }
    AGENT_PTY_MAP.delete(ptyId)
    log.info(`pty.destroy (agent): id=${ptyId}`)
    span.ok({ ptyId, graceful })                                                    // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { ptyId })                                                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.destroy failed: ${msg}` } }
  }
}
```

`handlePtySendSignal()` — chỉ mở span `terminal:destroy` khi signal tương đương kết thúc tiến trình (`SIGKILL`/`SIGTERM`); các tín hiệu điều khiển khác (`SIGINT`/`SIGHUP`/`SIGTSTP`) coi như hành vi tương tác, không trace — cùng lý do với `pty.write`/`agent.sendInput` (không phải Ctrl+C) ở SOL-AG-TRACE-002:

```typescript
export async function handlePtySendSignal(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const ptyId  = typeof params.id     === 'string' ? params.id     : ''
  const signal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  const isTerminating = signal === 'SIGKILL' || signal === 'SIGTERM'                // [NEW]
  const span = isTerminating                                                        // [NEW]
    ? Tracers.terminalDestroy.start({ origin: 'agent-pty', ptyId, signal, via: 'pty.sendSignal' }, resumeFrom(params))
    : undefined

  if (!ptyId) {
    span?.fail('missing id')                                                        // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.sendSignal: missing id' } }
  }
  if (!ALLOWED_SIGNALS.has(signal)) {
    span?.fail(`signal not allowed: ${signal}`)                                     // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: `Signal not allowed: ${signal}` } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span?.fail('pty not found', { ptyId })                                          // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    if (process.platform !== 'win32') { entry.pty.kill(signal) }
    else { entry.pty.kill() }
    log.info(`pty.sendSignal (agent): id=${ptyId} signal=${signal}`)
    span?.ok({ ptyId, signal })                                                     // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span?.fail(err, { ptyId })                                                      // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.sendSignal failed: ${msg}` } }
  }
}
```

`handlePtyWrite()` và `handlePtyScrollback()` — **không sửa**, giữ nguyên như code hiện tại (xem quyết định thiết kế ở mục 1).

## 4. Test Plan (Vitest)

Chưa có file test cho `pty-agent-bridge.ts` trong `src/relay/__tests__/` — tạo mới `src/relay/__tests__/pty-agent-bridge.test.ts`, theo pattern isomorphic sink giống 2 solution trước:

```typescript
// src/relay/__tests__/pty-agent-bridge.test.ts (NEW)
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import {
  handlePtyCreate, handlePtyResize, handlePtyDestroy,
  handlePtySendSignal, handlePtyWrite, handlePtyScrollback,
} from '../pty-agent-bridge'

describe('pty-agent-bridge — terminal:* tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
  })
  afterEach(() => unregister())

  it('handlePtyCreate emits terminal:create span, ok() contains ptyId+shell+cwd', async () => { /* mock node-pty */ })
  it('handlePtyCreate emits fail() when node-pty import fails', async () => { /* mock import failure */ })
  it('handlePtyResize emits terminal:resize span with cols/rows', async () => { /* ... */ })
  it('handlePtyDestroy emits terminal:destroy span with graceful field', async () => { /* ... */ })
  it('handlePtyDestroy emits ok(alreadyDead=true) span when ptyId not registered', async () => { /* ... */ })

  it('handlePtySendSignal emits terminal:destroy span for SIGKILL', async () => { /* ... */ })
  it('handlePtySendSignal emits terminal:destroy span for SIGTERM', async () => { /* ... */ })
  it('handlePtySendSignal does NOT emit any span for SIGINT/SIGHUP/SIGTSTP', async () => {
    // assert events.filter(e => e.flow === 'terminal:destroy').length === 0
  })

  it('handlePtyWrite does NOT emit any trace span regardless of call count', async () => {
    for (let i = 0; i < 20; i++) await handlePtyWrite(i, { id: 'x', data: 'a' }, log)
    expect(events.filter(e => e.flow.startsWith('terminal:'))).toHaveLength(0)
  })

  it('handlePtyScrollback does NOT emit any trace span', async () => {
    await handlePtyScrollback(1, { id: 'x' }, log)
    expect(events.filter(e => e.flow.startsWith('terminal:'))).toHaveLength(0)
  })

  it('resumes span id from params._trace.id when present', async () => {
    // handlePtyResize with _trace: { id: 'resumed-1' } → assert start event id === 'resumed-1'
  })

  it('generates a new span id when params._trace is absent', async () => { /* ... */ })
})
```

## 5. Acceptance Criteria

- [ ] `Tracers.terminalCreate`/`terminalResize`/`terminalDestroy` tồn tại trong `tracers.ts` (idempotent nếu solution khác đã thêm)
- [ ] `handlePtyCreate()`, `handlePtyResize()`, `handlePtyDestroy()` trong `pty-agent-bridge.ts` phát span tương ứng, `ok()` chứa `ptyId` (và `cols/rows`/`graceful` tuỳ method), `fail()` khi lỗi (node-pty không có sẵn, PTY không tồn tại, exception khi spawn/resize/kill)
- [ ] Mọi span đều có field `origin: 'agent-pty'` để phân biệt với `terminal:create` phát sinh từ `LocalPtyProvider`/`SshPtyProvider` (nếu backend-side solution dùng chung tên tracer)
- [ ] `handlePtySendSignal()` chỉ phát `terminal:destroy` khi `signal` là `SIGKILL` hoặc `SIGTERM` — không phát span cho `SIGINT`/`SIGHUP`/`SIGTSTP`
- [ ] `handlePtyWrite()` và `handlePtyScrollback()` **không** phát bất kỳ span nào (xác nhận bằng test đếm span, theo nguyên tắc CR-TRACE-000 §5)
- [ ] Mọi span resume `id` từ `params._trace.id` khi có mặt, tự sinh id mới khi không có (forward-compatible vì hiện chưa có caller thật gửi `_trace` cho `pty.*`)
- [ ] `src/relay/__tests__/pty-agent-bridge.test.ts` (file mới) có đủ test case ở mục 4, pass với `vitest run`
- [ ] Không đụng đến `src/main/providers/local-pty-provider.ts`, `ssh-pty-provider.ts`, hay `terminal-scrollback-snapshots.ts` — các file này thuộc solution set khác
